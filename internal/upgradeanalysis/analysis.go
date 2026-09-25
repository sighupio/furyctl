// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package upgradeanalysis gathers what changes between the distribution version a
// cluster runs and a target version, hop by hop, so that an upgrade can be planned
// before anything is touched. It only reads: from the cluster, and from the
// distribution manifests of the versions along the way.
package upgradeanalysis

// Analysis is the full report for one cluster and one target version.
type Analysis struct {
	ClusterName string   `json:"clusterName"        yaml:"clusterName"`
	Kind        string   `json:"kind"               yaml:"kind"`
	From        string   `json:"from"               yaml:"from"`
	To          string   `json:"to"                 yaml:"to"`
	Hops        []Hop    `json:"hops"               yaml:"hops"`
	Deployed    []string `json:"deployed"           yaml:"deployed"`
	Skipped     []string `json:"skipped,omitempty"  yaml:"skipped,omitempty"`
	Warnings    []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// AlreadyAtTarget reports whether the cluster already runs the target version, in
// which case there is nothing to plan.
func (a *Analysis) AlreadyAtTarget() bool {
	return len(a.Hops) == 0
}

// Hop is one supported upgrade step, with everything it changes.
type Hop struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to"   yaml:"to"`

	// KubernetesFrom and KubernetesTo are empty for kinds where furyctl does not
	// manage Kubernetes itself, such as KFDDistribution.
	KubernetesFrom string `json:"kubernetesFrom,omitempty" yaml:"kubernetesFrom,omitempty"`
	KubernetesTo   string `json:"kubernetesTo,omitempty"   yaml:"kubernetesTo,omitempty"`
	InstallerFrom  string `json:"installerFrom,omitempty"  yaml:"installerFrom,omitempty"`
	InstallerTo    string `json:"installerTo,omitempty"    yaml:"installerTo,omitempty"`

	// DistributionOnly marks a hop that does not touch Kubernetes at all: the version
	// does not move and furyctl ships no pre-kubernetes script for it.
	DistributionOnly bool `json:"distributionOnly" yaml:"distributionOnly"`

	Modules []ModuleDelta `json:"modules" yaml:"modules"`

	// Findings are the configuration changes this hop requires, found by comparing the
	// schema of the two versions against the configuration the cluster stores.
	Findings []Finding `json:"findings,omitempty" yaml:"findings,omitempty"`
}

// KubernetesChanged reports whether the Kubernetes version moves in this hop.
func (h *Hop) KubernetesChanged() bool {
	return h.KubernetesFrom != h.KubernetesTo
}

// ChangedModules returns only the modules whose version moves in this hop.
func (h *Hop) ChangedModules() []ModuleDelta {
	changed := make([]ModuleDelta, 0, len(h.Modules))

	for _, m := range h.Modules {
		if m.Changed() {
			changed = append(changed, m)
		}
	}

	return changed
}

// UnchangedModules returns the names of the modules that stay put in this hop.
func (h *Hop) UnchangedModules() []string {
	var unchanged []string

	for _, m := range h.Modules {
		if !m.Changed() {
			unchanged = append(unchanged, m.Name)
		}
	}

	return unchanged
}

// ModuleDelta is the version change of one module that the cluster actually deploys.
type ModuleDelta struct {
	// Name is the display name, matching the one `furyctl get cluster-info` shows.
	Name string `json:"name" yaml:"name"`
	// Type is the variant deployed on this cluster, for example cilium or loki.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	From string `json:"from"           yaml:"from"`
	To   string `json:"to"             yaml:"to"`
}

// Changed reports whether the module version moves.
func (m *ModuleDelta) Changed() bool {
	return m.From != m.To
}
