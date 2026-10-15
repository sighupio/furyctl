// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"github.com/sighupio/furyctl/internal/cluster"
)

const (
	// ReasonNoPlanForPhase is the reason of the resource sections of a phase that has no plan to show.
	ReasonNoPlanForPhase = "no plan available for this phase"

	reasonNoCluster = "no cluster found"
)

// KindDiff is what a cluster kind gives to RunDiff. The functions are the kind's own steps; RunDiff only decides
// which of them run and what the sections say when one cannot.
type KindDiff struct {
	// Phase is the phase flag of the command, cluster.OperationPhaseAll when it is not set.
	Phase string
	// RenderedConfig is the configuration rendered with the distribution defaults.
	RenderedConfig map[string]any
	// PluginsFeature tells whether the distribution has the plugins feature.
	PluginsFeature bool
	// Preflight runs the kind's preflight, built with the apply defaults. It reports whether the cluster exists.
	Preflight func() (bool, error)
	// AfterPreflight runs once, right after the preflight attempt whatever its outcome, before any render.
	AfterPreflight func()
	// RenderDistribution renders the distribution phase.
	RenderDistribution func() error
	// DiffDistribution compares the rendered distribution phase.
	DiffDistribution func() Section
	// DiffPlugins compares the plugins phase.
	DiffPlugins func() ([]Section, error)
}

// RunDiff compares the resources of a cluster: the preflight, the render of the distribution phase and the diff
// commands run, and nothing that writes to the cluster does.
func RunDiff(in KindDiff) []Section {
	distribution, plugins := in.comparedParts()

	titles := []string{}

	if distribution {
		titles = append(titles, distributionTitle)
	}

	if plugins {
		titles = append(titles, in.pluginTitles()...)
	}

	if in.Phase == cluster.OperationPhaseInfrastructure || in.Phase == cluster.OperationPhaseKubernetes {
		in.afterPreflight()

		return uniformSections(in.allTitles(), StatusOutOfScope, ReasonNoPlanForPhase)
	}

	exists, err := in.Preflight()

	in.afterPreflight()

	if err != nil {
		return uniformSections(titles, StatusFailed, "preflight failed: "+collapseLines(err.Error()))
	}

	if !exists {
		return uniformSections(titles, StatusFailed, reasonNoCluster)
	}

	sections := []Section{}

	if distribution {
		sections = append(sections, in.diffDistribution())
	}

	if plugins {
		sections = append(sections, in.diffPlugins()...)
	}

	return sections
}

// OutOfScopeSections gives every resource section of a configuration the same out-of-scope reason, for a kind
// that cannot be compared. Nothing runs for them.
func OutOfScopeSections(renderedConfig map[string]any, pluginsFeature bool, reason string) []Section {
	in := KindDiff{RenderedConfig: renderedConfig, PluginsFeature: pluginsFeature}

	return uniformSections(in.allTitles(), StatusOutOfScope, reason)
}

// afterPreflight runs AfterPreflight, when it is set.
func (in KindDiff) afterPreflight() {
	if in.AfterPreflight != nil {
		in.AfterPreflight()
	}
}

// comparedParts tells whether the phase of the command compares the distribution and the plugins sections.
func (in KindDiff) comparedParts() (bool, bool) {
	all := in.Phase == cluster.OperationPhaseAll

	return all || in.Phase == cluster.OperationPhaseDistribution,
		(all || in.Phase == cluster.OperationPhasePlugins) && in.PluginsFeature
}

func (in KindDiff) diffDistribution() Section {
	if err := in.RenderDistribution(); err != nil {
		return Section{Title: distributionTitle, Status: StatusFailed, Reason: "render failed: " + collapseLines(err.Error())}
	}

	return in.DiffDistribution()
}

func (in KindDiff) diffPlugins() []Section {
	sections, err := in.DiffPlugins()
	if err != nil {
		return uniformSections(in.pluginTitles(), StatusFailed, "render failed: "+collapseLines(err.Error()))
	}

	return sections
}

// allTitles lists every section the configuration has.
func (in KindDiff) allTitles() []string {
	titles := []string{distributionTitle}

	if in.PluginsFeature {
		titles = append(titles, in.pluginTitles()...)
	}

	return titles
}

// pluginTitles lists the plugin sections the configuration has.
func (in KindDiff) pluginTitles() []string {
	titles := []string{}

	kustomize, helm := pluginKinds(in.RenderedConfig)

	if kustomize {
		titles = append(titles, KustomizeGroup)
	}

	if helm {
		titles = append(titles, HelmGroup)
	}

	return titles
}
