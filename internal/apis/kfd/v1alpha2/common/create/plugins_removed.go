// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	PluginKindKustomize = "kustomize"
	PluginKindHelm      = "helm"
)

// RemovedPlugin is a plugin stored in kube-system/furyctl-config and absent from the new configuration.
// Namespace is set for Helm releases only.
type RemovedPlugin struct {
	Kind      string
	Name      string
	Namespace string
}

// Target is the argument of prune.sh that names the plugin.
func (r RemovedPlugin) Target() string {
	if r.Kind == PluginKindHelm {
		return PluginKindHelm + ":" + r.Namespace + "/" + r.Name
	}

	return PluginKindKustomize + ":" + r.Name
}

func (r RemovedPlugin) String() string {
	if r.Kind == PluginKindHelm {
		return fmt.Sprintf("%s (helm release, namespace %s)", r.Name, r.Namespace)
	}

	return r.Name + " (kustomize)"
}

// RemovedPluginsError stops the apply until the user decides what happens to the removed plugins.
type RemovedPluginsError struct {
	Plugins []RemovedPlugin
}

func (e *RemovedPluginsError) Error() string {
	var b strings.Builder

	b.WriteString("These plugins were removed from spec.plugins, but their resources are still in the cluster:\n")

	for _, p := range e.Plugins {
		b.WriteString("  - " + p.String() + "\n")
	}

	b.WriteString("\nfuryctl stopped before changing anything. To continue, choose one:\n" +
		"  - Delete their resources:                furyctl apply --prune-plugins\n" +
		"  - Keep them managed by furyctl:          add the entries back to spec.plugins\n" +
		"  - Keep the resources outside of furyctl: furyctl apply --force plugins-unmanage")

	return b.String()
}

// PreFlightError wraps an error of a preflight phase, except the removed-plugins error: the spec wants its
// text verbatim, without other error prefixes.
func PreFlightError(err error) error {
	if removedErr, ok := errors.AsType[*RemovedPluginsError](err); ok {
		return removedErr
	}

	return fmt.Errorf("error while executing preflight phase: %w", err)
}

// pluginsConfig, pluginsSpec, pluginsPlugins and pluginsHelm hold only the plugin identities of a configuration.
type pluginsConfig struct {
	Spec pluginsSpec `yaml:"spec"`
}

type pluginsSpec struct {
	Plugins pluginsPlugins `yaml:"plugins"`
}

type pluginsPlugins struct {
	Kustomize []pluginIdentity `yaml:"kustomize"`
	Helm      pluginsHelm      `yaml:"helm"`
}

type pluginsHelm struct {
	Releases []pluginIdentity `yaml:"releases"`
}

type pluginIdentity struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// identities lists the plugins of a configuration by name, kustomize first, each group in configuration order.
func identities(cfg pluginsConfig) []RemovedPlugin {
	ids := make([]RemovedPlugin, 0, len(cfg.Spec.Plugins.Kustomize)+len(cfg.Spec.Plugins.Helm.Releases))

	for _, k := range cfg.Spec.Plugins.Kustomize {
		ids = append(ids, RemovedPlugin{Kind: PluginKindKustomize, Name: k.Name})
	}

	for _, r := range cfg.Spec.Plugins.Helm.Releases {
		ids = append(ids, RemovedPlugin{Kind: PluginKindHelm, Name: r.Name, Namespace: r.Namespace})
	}

	return ids
}

// RemovedPlugins compares by name, not by list position: reducers compare paths by index and would report
// a reordered list as a change.
func RemovedPlugins(stored, current []byte) ([]RemovedPlugin, error) {
	var storedCfg, currentCfg pluginsConfig

	if err := yamlx.UnmarshalV3(stored, &storedCfg); err != nil {
		return nil, fmt.Errorf("error while reading the plugins of the stored configuration: %w", err)
	}

	if err := yamlx.UnmarshalV3(current, &currentCfg); err != nil {
		return nil, fmt.Errorf("error while reading the plugins of the new configuration: %w", err)
	}

	kept := identities(currentCfg)

	removed := []RemovedPlugin{}

	for _, id := range identities(storedCfg) {
		if !slices.Contains(kept, id) {
			removed = append(removed, id)
		}
	}

	return removed, nil
}

// CheckRemovedPlugins is the preflight check shared by every kind. Without a stored configuration it does
// nothing, like the other preflight checks. With prunePlugins the removed plugins go to the prune step.
// The returned error is meant to reach the user unwrapped.
func CheckRemovedPlugins( //nolint:revive // The flag decides whether the removed plugins block the apply.
	stateStore state.Storer,
	renderedConfig map[string]any,
	force []string,
	prunePlugins bool,
) ([]RemovedPlugin, error) {
	stored, err := stateStore.GetConfig()
	if err != nil {
		logrus.Debug("Cannot find state in cluster, skipping removed plugins check: ", err)

		return nil, nil
	}

	current, err := yamlx.MarshalV3(renderedConfig)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling the new configuration: %w", err)
	}

	removed, err := RemovedPlugins(stored, current)
	if err != nil || len(removed) == 0 {
		return removed, err
	}

	// --prune-plugins wins over the catch-all --force all; the apply command rejects it with --force plugins-unmanage.
	if prunePlugins {
		return removed, nil
	}

	if !cluster.IsForceEnabledForFeature(force, cluster.ForceFeaturePluginsUnmanage) {
		return removed, &RemovedPluginsError{Plugins: removed}
	}

	names := make([]string, len(removed))
	for i, r := range removed {
		names[i] = r.String()
	}

	logrus.Warnf(
		"These plugins were removed from spec.plugins and furyctl no longer manages them, "+
			"their resources stay in the cluster unmanaged: %s",
		strings.Join(names, ", "),
	)

	return removed, nil
}
