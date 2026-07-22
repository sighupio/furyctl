// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterinfo

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/sighupio/furyctl/configs"
	distroconf "github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	furyctlConfigSecret   = "furyctl-config"
	furyctlKFDSecret      = "furyctl-kfd"
	upgradeStateConfigMap = "furyctl-upgrade-state"
	kubeSystemNamespace   = "kube-system"

	roleControlPlane = "control-plane"
	roleMaster       = "master"
	roleNone         = "<none>"
	ingressNone      = "none"
	thousandDec      = 1000.0
	thousandBin      = 1024.0
	milliCPU         = 1000
	etcdStacked      = "Stacked"
	etcdDedicated    = "Dedicated"
)

var (
	ErrSecretNoData         = errors.New("secret has no data field")
	ErrSecretMissingKey     = errors.New("secret missing key")
	ErrConfigSecretNotFound = errors.New("furyctl-config secret not found in the cluster")
	ErrUpgradeStateNoData   = errors.New("upgrade state configmap has no data")
	ErrUpgradeStateMissing  = errors.New("upgrade state configmap missing state key")
)

// Collector reads cluster information from the Kubernetes secrets and configmaps
// that furyctl maintains during cluster lifecycle operations.
type Collector struct {
	KubectlRunner *kubectl.Runner
}

// NewCollector creates a Collector using the given kubectl binary and working directory.
func NewCollector(kubectlBin, workDir string) *Collector {
	runner := kubectl.NewRunner(
		execx.NewStdExecutor(),
		kubectl.Paths{
			Kubectl: kubectlBin,
			WorkDir: workDir,
		},
		false, true, false,
	)

	return &Collector{KubectlRunner: runner}
}

// Collect gathers all available cluster information and returns a populated struct.
// Failures fetching the Kubernetes version or node list are non-fatal: those fields are
// left empty so the command can still show partial information.
func (c *Collector) Collect() (*Info, error) {
	rawConfig, configTimestamp, err := c.fetchSecret(furyctlConfigSecret, "config")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigSecretNotFound, err)
	}

	furyctlConf := distroconf.Furyctl{}
	if err := yamlx.UnmarshalV3(rawConfig, &furyctlConf); err != nil {
		return nil, fmt.Errorf("error while parsing stored cluster configuration: %w", err)
	}

	configMap := map[string]any{}
	if err := yamlx.UnmarshalV3(rawConfig, &configMap); err != nil {
		return nil, fmt.Errorf("error while parsing stored cluster configuration map: %w", err)
	}

	rawSD, _, err := c.fetchSecret(furyctlKFDSecret, "kfd")
	if err != nil {
		return nil, fmt.Errorf("error while reading KFD YAML file from cluster: %w", err)
	}

	sdManifest := distroconf.KFD{}
	if err := yamlx.UnmarshalV3(rawSD, &sdManifest); err != nil {
		return nil, fmt.Errorf("error while parsing KFD YAML file: %w", err)
	}

	info := &Info{
		ClusterName:             furyctlConf.Metadata.Name,
		SDVersion:               furyctlConf.Spec.DistributionVersion,
		SDKind:                  furyctlConf.Kind,
		SDInstallerVersion:      installerVersion(furyctlConf.Kind, sdManifest),
		SDUpgradePaths:          computeUpgradePaths(furyctlConf.Kind, furyctlConf.Spec.DistributionVersion),
		LastConfigurationChange: configTimestamp,
		CustomPatchesPresent:    hasCustomPatches(configMap),
		Modules:                 extractModules(configMap, sdManifest, furyctlConf.Kind),
		Plugins:                 extractPlugins(configMap),
		EtcdTopology:            etcdTopology(furyctlConf.Kind, configMap),
	}

	if ongoingUpgrade, upgradeErr := c.fetchOngoingUpgrade(); upgradeErr == nil {
		info.SDOngoingUpgrade = ongoingUpgrade
	}

	if k8sVersion, versionErr := c.fetchKubernetesVersion(); versionErr == nil {
		info.KubernetesVersion = k8sVersion
	}

	if nodes, nodesErr := c.fetchNodes(); nodesErr == nil {
		info.Nodes = nodes
	}

	return info, nil
}

func (c *Collector) fetchSecret(secretName, dataKey string) ([]byte, time.Time, error) {
	out, err := c.KubectlRunner.Get(
		true,
		kubeSystemNamespace,
		"secret", secretName,
		"-o", "yaml",
		"--show-managed-fields",
	)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("error reading secret %s: %w", secretName, err)
	}

	raw := map[string]any{}
	if err := yamlx.UnmarshalV3([]byte(out), &raw); err != nil {
		return nil, time.Time{}, fmt.Errorf("error parsing secret %s: %w", secretName, err)
	}

	data, ok := raw["data"].(map[string]any)
	if !ok {
		return nil, time.Time{}, fmt.Errorf("%w: %s", ErrSecretNoData, secretName)
	}

	encoded, ok := data[dataKey].(string)
	if !ok {
		return nil, time.Time{}, fmt.Errorf("%w %q in %s", ErrSecretMissingKey, dataKey, secretName)
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("error decoding secret %s key %q: %w", secretName, dataKey, err)
	}

	return decoded, latestManagedFieldTime(raw), nil
}

// fetchOngoingUpgrade reads the upgrade state configmap and returns an OngoingUpgrade
// when an upgrade is currently in progress or has a failed phase.
func (c *Collector) fetchOngoingUpgrade() (*OngoingUpgrade, error) {
	out, err := c.KubectlRunner.Get(
		false,
		kubeSystemNamespace,
		"cm", upgradeStateConfigMap,
		"-o", "yaml",
	)
	if err != nil {
		return nil, fmt.Errorf("upgrade state configmap not found: %w", err)
	}

	raw := map[string]any{}
	if err := yamlx.UnmarshalV3([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("error parsing upgrade state: %w", err)
	}

	cmData, ok := raw["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w", ErrUpgradeStateNoData)
	}

	stateYAML, ok := cmData["state"].(string)
	if !ok {
		return nil, fmt.Errorf("%w", ErrUpgradeStateMissing)
	}

	state := &upgrade.State{}
	if err := yamlx.UnmarshalV3([]byte(stateYAML), state); err != nil {
		return nil, fmt.Errorf("error parsing upgrade state YAML: %w", err)
	}

	return upgradeInfoFromState(state), nil
}

// fetchKubernetesVersion retrieves the Kubernetes server version.
func (c *Collector) fetchKubernetesVersion() (string, error) {
	out, err := c.KubectlRunner.Version()
	if err != nil {
		return "", fmt.Errorf("error getting kubernetes version: %w", err)
	}

	type versionInfo struct {
		GitVersion string `json:"gitVersion"`
	}

	type kubectlVersion struct {
		ServerVersion versionInfo `json:"serverVersion"`
	}

	var info kubectlVersion
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return "", fmt.Errorf("error parsing kubernetes version response: %w", err)
	}

	return info.ServerVersion.GitVersion, nil
}

// fetchNodes summarizes node capacity by role to report cluster shape.
func (c *Collector) fetchNodes() (*NodesSummary, error) {
	out, err := c.KubectlRunner.Get(
		false,
		"all",
		"nodes",
		"-o", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("error getting nodes: %w", err)
	}

	type nodeResource struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	}

	type nodeStatus struct {
		Capacity nodeResource `json:"capacity"`
	}

	type nodeMetadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	}

	type nodeItem struct {
		Metadata nodeMetadata `json:"metadata"`
		Status   nodeStatus   `json:"status"`
	}

	type nodeList struct {
		Items []nodeItem `json:"items"`
	}

	var list nodeList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("error parsing nodes JSON: %w", err)
	}

	if len(list.Items) == 0 {
		return &NodesSummary{}, nil
	}

	groups := map[string]*NodeRoleGroup{}

	var roleOrder []string

	totals := NodeTotals{}

	for _, item := range list.Items {
		role := primaryRole(item.Metadata.Labels)
		vcpu := parseCPU(item.Status.Capacity.CPU)
		ramGb := parseMemoryGb(item.Status.Capacity.Memory)

		if _, exists := groups[role]; !exists {
			groups[role] = &NodeRoleGroup{Role: role}
			roleOrder = append(roleOrder, role)
		}

		groups[role].Quantity++
		groups[role].VCPU += vcpu
		groups[role].RAMGb += ramGb

		totals.Quantity++
		totals.VCPU += vcpu
		totals.RAMGb += ramGb
	}

	sort.Slice(roleOrder, func(i, j int) bool {
		return roleSort(roleOrder[i], roleOrder[j])
	})

	roles := make([]NodeRoleGroup, 0, len(roleOrder))
	for _, r := range roleOrder {
		roles = append(roles, *groups[r])
	}

	return &NodesSummary{Roles: roles, Totals: totals}, nil
}

// latestManagedFieldTime returns the most recent managedFields[].time,
// falling back to creationTimestamp.
func latestManagedFieldTime(raw map[string]any) time.Time {
	metadata, ok := raw["metadata"].(map[string]any)
	if !ok {
		return time.Time{}
	}

	if fields, ok := metadata["managedFields"].([]any); ok {
		var latest time.Time

		for _, f := range fields {
			field, ok := f.(map[string]any)
			if !ok {
				continue
			}

			ts, ok := field["time"].(string)
			if !ok {
				continue
			}

			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}

			if t.After(latest) {
				latest = t
			}
		}

		if !latest.IsZero() {
			return latest
		}
	}

	if ts, ok := metadata["creationTimestamp"].(string); ok {
		t, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			return t
		}
	}

	return time.Time{}
}

// upgradeInfoFromState returns the first pending/failed phase in canonical
// order; nil if all succeeded.
func upgradeInfoFromState(state *upgrade.State) *OngoingUpgrade {
	for _, phaseName := range cluster.GetPhasesOrder() {
		reflectedPhase := reflect.ValueOf(state.Phases).FieldByName(phaseName)
		if !reflectedPhase.IsValid() || reflectedPhase.IsNil() {
			continue
		}

		status := reflectedPhase.Elem().FieldByName("Status").String()
		if status == string(upgrade.PhaseStatusPending) || status == string(upgrade.PhaseStatusFailed) {
			return &OngoingUpgrade{
				Status: status,
				Phase:  cluster.GetPhase(phaseName),
			}
		}
	}

	return nil
}

func hasCustomPatches(configMap map[string]any) bool {
	cp := nestedMap(configMap, "spec", "distribution", "customPatches")
	if cp == nil {
		return false
	}

	for _, v := range cp {
		if slice, ok := v.([]any); ok && len(slice) > 0 {
			return true
		}
	}

	return false
}

// extractModules combines types from furyctl config with versions from the KFD
// YAML. Fixed order keeps output stable; for EKS, an AWS row is appended when
// present.
func extractModules(configMap map[string]any, sd distroconf.KFD, kind string) []ModuleInfo {
	modules := nestedMap(configMap, "spec", "distribution", "modules")

	type moduleSpec struct {
		name       string
		version    string
		typeGetter func(map[string]any) string
	}

	specs := []moduleSpec{
		{
			name:    "Networking",
			version: sd.Modules.Networking,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(m, "networking"), "type")
			},
		},
		{
			name:       "Ingress",
			version:    sd.Modules.Ingress,
			typeGetter: ingressType,
		},
		{
			name:    "Monitoring",
			version: sd.Modules.Monitoring,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(m, "monitoring"), "type")
			},
		},
		{
			name:    "Logging",
			version: sd.Modules.Logging,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(m, "logging"), "type")
			},
		},
		{
			name:    "Tracing",
			version: sd.Modules.Tracing,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(m, "tracing"), "type")
			},
		},
		{
			name:    "Policy",
			version: sd.Modules.Opa,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(m, "policy"), "type")
			},
		},
		{
			name:    "Auth",
			version: sd.Modules.Auth,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(nestedMap(m, "auth"), "provider"), "type")
			},
		},
		{
			name:    "Disaster Recovery",
			version: sd.Modules.Dr,
			typeGetter: func(m map[string]any) string {
				return stringField(nestedMap(m, "dr"), "type")
			},
		},
	}

	if sd.Modules.Aws != "" && kind == "EKSCluster" {
		specs = append(specs, moduleSpec{
			name:       "AWS",
			version:    sd.Modules.Aws,
			typeGetter: func(_ map[string]any) string { return "" },
		})
	}

	result := make([]ModuleInfo, 0, len(specs))

	for _, s := range specs {
		modType := ""
		if modules != nil {
			modType = s.typeGetter(modules)
		}

		result = append(result, ModuleInfo{
			Name:    s.name,
			Version: s.version,
			Type:    modType,
		})
	}

	return result
}

// extractPlugins builds the grouped PluginsInfo from the stored configuration,
// separating Kustomize and Helm plugin types.
func extractPlugins(configMap map[string]any) *PluginsInfo {
	plugins := nestedMap(configMap, "spec", "plugins")
	if plugins == nil {
		return nil
	}

	result := &PluginsInfo{}
	hasAny := false

	if kustomize, ok := plugins["kustomize"].([]any); ok {
		for _, item := range kustomize {
			if entry, ok := item.(map[string]any); ok {
				if name, ok := entry["name"].(string); ok {
					result.Kustomize = append(result.Kustomize, name)
					hasAny = true
				}
			}
		}
	}

	if helm, ok := plugins["helm"].(map[string]any); ok {
		if releases, ok := helm["releases"].([]any); ok {
			for _, item := range releases {
				if entry, ok := item.(map[string]any); ok {
					if name, ok := entry["name"].(string); ok {
						result.Helm = append(result.Helm, name)
						hasAny = true
					}
				}
			}
		}
	}

	if !hasAny {
		return nil
	}

	return result
}

func etcdTopology(kind string, configMap map[string]any) string {
	switch kind {
	case "OnPremises":
		return onPremisesEtcdTopology(configMap)

	case "Immutable":
		return immutableEtcdTopology(configMap)

	default:
		return ""
	}
}

// onPremisesEtcdTopology returns Dedicated when spec.kubernetes.etcd.hosts is
// non-empty, otherwise Stacked.
func onPremisesEtcdTopology(configMap map[string]any) string {
	etcd := nestedMap(configMap, "spec", "kubernetes", "etcd")
	if etcd == nil {
		return etcdStacked
	}

	hosts, ok := etcd["hosts"].([]any)
	if !ok || len(hosts) == 0 {
		return etcdStacked
	}

	return etcdDedicated
}

// immutableEtcdTopology returns Stacked when etcd members are a subset of
// controlPlane members, otherwise Dedicated.
func immutableEtcdTopology(configMap map[string]any) string {
	etcdHosts := memberHostnames(nestedMap(configMap, "spec", "kubernetes", "etcd"))
	if len(etcdHosts) == 0 {
		return etcdStacked
	}

	cpHosts := memberHostnames(nestedMap(configMap, "spec", "kubernetes", "controlPlane"))

	cpSet := make(map[string]struct{}, len(cpHosts))
	for _, h := range cpHosts {
		cpSet[h] = struct{}{}
	}

	for _, h := range etcdHosts {
		if _, ok := cpSet[h]; !ok {
			return etcdDedicated
		}
	}

	return etcdStacked
}

func memberHostnames(section map[string]any) []string {
	if section == nil {
		return nil
	}

	members, ok := section["members"].([]any)
	if !ok {
		return nil
	}

	hostnames := make([]string, 0, len(members))

	for _, m := range members {
		entry, ok := m.(map[string]any)
		if !ok {
			continue
		}

		if h := stringField(entry, "hostname"); h != "" {
			hostnames = append(hostnames, h)
		}
	}

	return hostnames
}

func installerVersion(kind string, sd distroconf.KFD) string {
	switch kind {
	case "OnPremises":
		return sd.Kubernetes.OnPremises.Installer

	case "EKSCluster":
		return sd.Kubernetes.Eks.Installer

	case "Immutable":
		return sd.Kubernetes.Immutable.Installer

	default:
		return ""
	}
}

// computeUpgradePaths returns the list of available upgrade target versions for the given
// cluster kind and current distribution version, using the embedded upgrade paths filesystem.
func computeUpgradePaths(kind, fromVersion string) []string {
	from := strings.TrimPrefix(fromVersion, "v")

	globPattern := fmt.Sprintf("upgrades/%s/%s-*", strings.ToLower(kind), from)

	matches, err := fs.Glob(configs.Tpl, globPattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	targets := make([]string, 0, len(matches))

	for _, match := range matches {
		info, err := fs.Stat(configs.Tpl, match)
		if err != nil || !info.IsDir() {
			continue
		}

		parts := strings.Split(filepath.Base(match), "-")
		to := parts[len(parts)-1]
		targets = append(targets, "v"+to)
	}

	return targets
}

// primaryRole returns the display role for a node by inspecting its labels.
// Control-plane and master roles take priority over any other role.
// Otherwise the first role label in alphabetical order is used.
// Returns "<none>" when no role labels are present, consistent with kubectl.
func primaryRole(labels map[string]string) string {
	const prefix = "node-role.kubernetes.io/"

	var roles []string

	for k := range labels {
		if role, ok := strings.CutPrefix(k, prefix); ok && role != "" {
			roles = append(roles, role)
		}
	}

	if len(roles) == 0 {
		return roleNone
	}

	sort.Slice(roles, func(i, j int) bool { return roleSort(roles[i], roles[j]) })

	return roles[0]
}

func roleSort(a, b string) bool {
	if a == roleControlPlane {
		return true
	}

	if b == roleControlPlane {
		return false
	}

	if a == roleMaster {
		return true
	}

	if b == roleMaster {
		return false
	}

	if a == roleNone {
		return false
	}

	if b == roleNone {
		return true
	}

	return a < b
}

// parseCPU converts a Kubernetes CPU quantity string (e.g. "4" or "500m") to an integer vCPU count.
func parseCPU(cpu string) int64 {
	if cpu == "" {
		return 0
	}

	if strings.HasSuffix(cpu, "m") {
		var millis int64

		if _, err := fmt.Sscanf(strings.TrimSuffix(cpu, "m"), "%d", &millis); err != nil {
			return 0
		}

		return millis / milliCPU
	}

	var cores int64

	if _, err := fmt.Sscanf(cpu, "%d", &cores); err != nil {
		return 0
	}

	return cores
}

// parseMemoryGb converts a Kubernetes memory quantity string to a GiB value for display.
func parseMemoryGb(mem string) float64 {
	if mem == "" {
		return 0
	}

	suffixes := []struct {
		suffix string
		factor float64
	}{
		{"Ki", 1.0 / (thousandBin * thousandBin)},
		{"Mi", 1.0 / thousandBin},
		{"Gi", 1.0},
		{"Ti", thousandBin},
		{"K", 1.0 / (thousandDec * thousandDec)},
		{"M", 1.0 / thousandDec},
		{"G", 1.0},
		{"T", thousandDec},
	}

	for _, s := range suffixes {
		if strings.HasSuffix(mem, s.suffix) {
			var val float64

			if _, err := fmt.Sscanf(strings.TrimSuffix(mem, s.suffix), "%f", &val); err != nil {
				return 0
			}

			return val * s.factor
		}
	}

	var val float64

	if _, err := fmt.Sscanf(mem, "%f", &val); err != nil {
		return 0
	}

	return val / (thousandBin * thousandBin * thousandBin)
}

func nestedMap(m map[string]any, keys ...string) map[string]any {
	if m == nil {
		return nil
	}

	current := m

	for _, k := range keys {
		next, ok := current[k].(map[string]any)
		if !ok {
			return nil
		}

		current = next
	}

	return current
}

// ingressType derives a readable ingress type from the modules map.
// Returns a comma-separated list of active types, or "none" if all are disabled.
func ingressType(modules map[string]any) string {
	var active []string

	if t := stringField(nestedMap(modules, "ingress", "nginx"), "type"); t != "" && t != ingressNone {
		active = append(active, "nginx/"+t)
	}

	if t := stringField(nestedMap(modules, "ingress", "haproxy"), "type"); t != "" && t != ingressNone {
		active = append(active, "haproxy/"+t)
	}

	if byoic := nestedMap(modules, "ingress", "byoic"); byoic != nil {
		if enabled, ok := byoic["enabled"].(bool); ok && enabled {
			if class := stringField(byoic, "ingressClass"); class != "" {
				active = append(active, "byoic/"+class)
			} else {
				active = append(active, "byoic")
			}
		}
	}

	if len(active) == 0 {
		return ingressNone
	}

	return strings.Join(active, ", ")
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}

	if v, ok := m[key].(string); ok {
		return v
	}

	return ""
}
