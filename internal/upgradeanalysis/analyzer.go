// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/upgradepath"
)

const (
	// Module type a cluster reports for a module it does not deploy.
	moduleTypeNone = "none"
	// File furyctl ships for the hops that touch the Kubernetes phase.
	preKubernetesScript = "pre-kubernetes.sh.tpl"
)

// ErrNoManifest is returned when a version in the chain has no distribution manifest.
var ErrNoManifest = errors.New("no distribution manifest available for version")

// Distribution is one version of the distribution: its manifest, and the local directory
// holding the rest of it, such as the schemas.
type Distribution struct {
	Manifest config.KFD
	Path     string
}

// Fetcher provides the distribution of one version. It exists so the analysis logic can be
// exercised without downloading anything.
type Fetcher interface {
	Fetch(kind, version string) (Distribution, error)
}

// Build assembles the analysis for a cluster, given its current state and a target version.
// It resolves the hop chain, then describes what each hop changes, keeping only the modules
// the cluster actually deploys.
func Build(
	fsys fs.FS,
	fetcher Fetcher,
	info *clusterinfo.Info,
	cfg map[string]any,
	to string,
) (*Analysis, error) {
	chain, err := upgradepath.ResolveChain(fsys, info.SDKind, info.SDVersion, to)
	if err != nil {
		return nil, fmt.Errorf("cannot plan the upgrade: %w", err)
	}

	deployed, skipped := splitDeployed(info.Modules)

	analysis := &Analysis{
		ClusterName: info.ClusterName,
		Kind:        info.SDKind,
		From:        semver.EnsurePrefix(info.SDVersion),
		To:          semver.EnsurePrefix(to),
		Hops:        make([]Hop, 0, len(chain)),
		Deployed:    moduleNames(deployed),
		Skipped:     skipped,
	}

	// Nothing to plan: the cluster already runs the target version.
	if len(chain) == 0 {
		return analysis, nil
	}

	distributions, err := fetchAll(fetcher, info.SDKind, chain)
	if err != nil {
		return nil, err
	}

	for _, hop := range chain {
		built := buildHop(fsys, info.SDKind, hop, distributions, deployed)

		// Configuration findings need the schemas of both ends of the hop. A failure here
		// must not sink the whole report: the version deltas above are still worth having,
		// so the problem is reported as a warning instead.
		if cfg != nil {
			findings, err := configFindings(
				distributions[hop.From], distributions[hop.To], info.SDKind, cfg, hop.To,
			)
			if err != nil {
				analysis.Warnings = append(analysis.Warnings, fmt.Sprintf(
					"could not compare the configuration schemas for %s -> %s: %v", hop.From, hop.To, err,
				))
			}

			built.Findings = findings
		}

		analysis.Hops = append(analysis.Hops, built)
	}

	analysis.Warnings = append(analysis.Warnings, compatibilityWarnings(info.SDKind, chain)...)

	if cfg != nil {
		last := chain[len(chain)-1]
		for _, finding := range validationFindings(distributions[last.To], info.SDKind, cfg, last.To) {
			analysis.Warnings = append(analysis.Warnings, finding.Message)
		}
	}

	return analysis, nil
}

// fetchAll retrieves every version the chain touches, the starting one included, so that
// each hop can be described as a before and an after.
func fetchAll(fetcher Fetcher, kind string, chain []upgradepath.Hop) (map[string]Distribution, error) {
	distributions := map[string]Distribution{}

	for _, hop := range chain {
		for _, version := range []string{hop.From, hop.To} {
			if _, done := distributions[version]; done {
				continue
			}

			dist, err := fetcher.Fetch(kind, version)
			if err != nil {
				return nil, fmt.Errorf("%w %s: %w", ErrNoManifest, version, err)
			}

			distributions[version] = dist
		}
	}

	return distributions, nil
}

// buildHop describes a single hop: what Kubernetes does, and which deployed modules move.
func buildHop(
	fsys fs.FS,
	kind string,
	hop upgradepath.Hop,
	distributions map[string]Distribution,
	deployed []clusterinfo.ModuleInfo,
) Hop {
	from, to := distributions[hop.From].Manifest, distributions[hop.To].Manifest

	k8sFrom := kubernetesVersion(from, kind)
	k8sTo := kubernetesVersion(to, kind)

	return Hop{
		From:             hop.From,
		To:               hop.To,
		KubernetesFrom:   k8sFrom,
		KubernetesTo:     k8sTo,
		InstallerFrom:    installerVersion(from, kind),
		InstallerTo:      installerVersion(to, kind),
		DistributionOnly: isDistributionOnly(fsys, kind, hop, k8sFrom, k8sTo),
		Modules:          moduleDeltas(from, to, kind, deployed),
	}
}

// moduleDeltas lines up the module versions of two manifests, keeping only the modules the
// cluster deploys, in the order the cluster reports them.
func moduleDeltas(from, to config.KFD, kind string, deployed []clusterinfo.ModuleInfo) []ModuleDelta {
	fromVersions := clusterinfo.ModuleVersions(from, kind)
	toVersions := clusterinfo.ModuleVersions(to, kind)

	deltas := make([]ModuleDelta, 0, len(deployed))

	for _, module := range deployed {
		deltas = append(deltas, ModuleDelta{
			Name: module.Name,
			Type: module.Type,
			From: fromVersions[module.Name],
			To:   toVersions[module.Name],
		})
	}

	return deltas
}

// isDistributionOnly reports whether a hop leaves Kubernetes alone. That is the case when
// the Kubernetes version does not move and furyctl ships no pre-kubernetes script for the
// hop, which together mean only the distribution phase runs.
func isDistributionOnly(fsys fs.FS, kind string, hop upgradepath.Hop, k8sFrom, k8sTo string) bool {
	if k8sFrom != k8sTo {
		return false
	}

	// This kind has no Kubernetes phase for furyctl to run.
	if kind == distribution.KFDDistributionKind {
		return true
	}

	script := path.Join(
		"upgrades",
		strings.ToLower(kind),
		fmt.Sprintf("%s-%s", semver.EnsureNoPrefix(hop.From), semver.EnsureNoPrefix(hop.To)),
		preKubernetesScript,
	)

	_, err := fs.Stat(fsys, script)

	return err != nil
}

// splitDeployed separates the modules the cluster deploys from the ones it does not, so
// that the report can stay silent about everything that is switched off.
func splitDeployed(modules []clusterinfo.ModuleInfo) ([]clusterinfo.ModuleInfo, []string) {
	deployed := make([]clusterinfo.ModuleInfo, 0, len(modules))

	var skipped []string

	for _, module := range modules {
		if module.Type == moduleTypeNone {
			skipped = append(skipped, module.Name)

			continue
		}

		deployed = append(deployed, module)
	}

	return deployed, skipped
}

func moduleNames(modules []clusterinfo.ModuleInfo) []string {
	names := make([]string, 0, len(modules))

	for _, module := range modules {
		names = append(names, module.Name)
	}

	return names
}

// compatibilityWarnings reports the versions in the chain that this build of furyctl does
// not claim to support, so that the user learns it before starting rather than mid-upgrade.
func compatibilityWarnings(kind string, chain []upgradepath.Hop) []string {
	var warnings []string

	seen := map[string]bool{}

	for _, hop := range chain {
		for _, version := range []string{hop.From, hop.To} {
			if seen[version] {
				continue
			}

			seen[version] = true

			checker, err := distribution.NewCompatibilityChecker(version, kind)
			if err != nil || !checker.IsCompatible() {
				warnings = append(warnings, fmt.Sprintf(
					"SD %s is not supported by this version of furyctl; run `furyctl get supported-versions`",
					version,
				))
			}
		}
	}

	slices.Sort(warnings)

	return warnings
}

func kubernetesVersion(kfd config.KFD, kind string) string {
	switch kind {
	case distribution.OnPremisesKind:
		return kfd.Kubernetes.OnPremises.Version

	case distribution.EKSClusterKind:
		return kfd.Kubernetes.Eks.Version

	case distribution.ImmutableKind:
		return kfd.Kubernetes.Immutable.Version

	default:
		return ""
	}
}

func installerVersion(kfd config.KFD, kind string) string {
	switch kind {
	case distribution.OnPremisesKind:
		return kfd.Kubernetes.OnPremises.Installer

	case distribution.EKSClusterKind:
		return kfd.Kubernetes.Eks.Installer

	case distribution.ImmutableKind:
		return kfd.Kubernetes.Immutable.Installer

	default:
		return ""
	}
}
