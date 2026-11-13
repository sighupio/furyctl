// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
)

// RenderFunc renders templates/plugins and returns spec.plugins of the merged config, nil when it is missing.
type RenderFunc func() (map[any]any, error)

type PluginsRunners struct {
	Kapp     KappRunner
	Kubectl  KubectlRunner
	Helm     HelmRunner
	Helmfile HelmfileRunner
}

type Plugins struct {
	kappDeleter

	runners PluginsRunners
	render  RenderFunc
	enabled bool
}

func NewPlugins(runners PluginsRunners, render RenderFunc, kfdManifest config.KFD, dryRun bool) *Plugins {
	return &Plugins{
		kappDeleter: kappDeleter{kapp: runners.Kapp, dryRun: dryRun},
		runners:     runners,
		render:      render,
		enabled:     distribution.HasFeature(kfdManifest, distribution.FeaturePlugins),
	}
}

// Plugin is a kustomize plugin, deleted as the kapp app kfd-plugin-<Name>, or a helm release of spec.plugins.
type Plugin struct {
	Name string
	Helm bool

	release release
}

// Exec deletes the kustomize plugins, then the helm releases, each in reverse order of apply.
func (p *Plugins) Exec() error {
	plugins, err := p.List()
	if err != nil {
		return err
	}

	return p.Delete(plugins)
}

// List returns the plugins in delete order: kustomize plugins, then helm releases, each in reverse order of apply.
func (p *Plugins) List() ([]Plugin, error) {
	if !p.enabled {
		logrus.Info("Skipping the plugins delete: this distribution version has no plugins")

		return nil, nil
	}

	spec, err := p.render()
	if err != nil {
		return nil, err
	}

	kustomize, releases := readPlugins(spec)
	plugins := make([]Plugin, 0, len(kustomize)+len(releases))

	for _, name := range slices.Backward(kustomize) {
		plugins = append(plugins, Plugin{Name: name})
	}

	for _, r := range slices.Backward(releases) {
		plugins = append(plugins, Plugin{Name: r.name, Helm: true, release: r})
	}

	return plugins, nil
}

// Delete deletes the plugins in the given order.
func (p *Plugins) Delete(plugins []Plugin) error {
	for _, plugin := range plugins {
		if !plugin.Helm {
			logrus.Infof("Deleting plugin %s...", plugin.Name)

			if err := p.deleteApp(plugin.app(), nil); err != nil {
				return err
			}

			continue
		}

		if err := p.deleteRelease(plugin.release); err != nil {
			return err
		}
	}

	return nil
}

func (p Plugin) app() string {
	return "kfd-plugin-" + p.Name
}

// readPlugins reads the kustomize names and the helm releases of spec.plugins, in list order.
func readPlugins(spec map[any]any) ([]string, []release) {
	kustomize := maps(spec["kustomize"])
	names := make([]string, 0, len(kustomize))

	for _, m := range kustomize {
		names = append(names, str(m, "name"))
	}

	var releases []release

	if helm, ok := spec["helm"].(map[any]any); ok {
		for _, m := range maps(helm["releases"]) {
			releases = append(releases, release{
				name: str(m, "name"), namespace: str(m, "namespace"), chart: str(m, "chart"), version: str(m, "version"),
			})
		}
	}

	return names, releases
}

// maps returns the map entries of a YAML list, none when it is not a list.
func maps(list any) []map[any]any {
	items, ok := list.([]any)
	if !ok {
		return nil
	}

	var res []map[any]any

	for _, item := range items {
		if m, ok := item.(map[any]any); ok {
			res = append(res, m)
		}
	}

	return res
}

func str(m map[any]any, key string) string {
	if v := m[key]; v != nil {
		return fmt.Sprint(v)
	}

	return ""
}
