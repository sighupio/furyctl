// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
)

// Diff compares the plugins phase with the cluster after the render Exec does. It returns the sections of the
// kustomize plugins and of the helm releases of the configuration, none when the distribution has no plugins
// feature or the configuration defines no plugins.
func (p *Plugins) Diff() ([]Section, error) {
	if !distribution.HasFeature(p.kfd, distribution.FeaturePlugins) {
		return nil, nil
	}

	specPlugins, hasPlugins, err := p.render()
	if err != nil {
		return nil, err
	}

	if !hasPlugins {
		return nil, nil
	}

	sections := p.diffKustomize(pluginNames(specPlugins["kustomize"]))

	return append(sections, p.diffHelm(pluginNames(releasesOf(specPlugins)))...), nil
}

func (p *Plugins) diffKustomize(names []string) []Section {
	if len(names) == 0 {
		return nil
	}

	sections := make([]Section, 0, len(names))

	if _, supported := checkDiffSupport("", p.paths.DistroPath, cluster.OperationPhasePlugins); !supported {
		version := distributionVersion(p.paths.DistroPath)

		for _, name := range names {
			sections = append(sections, Section{
				Title: kustomizeTitle(name), Status: StatusNotSupported, Reason: "distribution " + version,
			})
		}

		return sections
	}

	out, code, err := runDiffScript(p.shellRunner, filepath.Join(p.Path, "scripts", "diff.sh"), p.Path)

	for _, name := range names {
		title := kustomizeTitle(name)

		switch {
		case err != nil:
			sections = append(sections, Section{Title: title, Status: StatusFailed, Reason: err.Error()})
		case code != 0:
			if clean, ok := cleanKappSection(title, filepath.Join(p.Path, "kapp-diff-"+name+".json")); ok {
				sections = append(sections, clean)

				continue
			}

			sections = append(sections, failedScript(title, code, out, p.Path))
		default:
			sections = append(sections, kappSection(title, filepath.Join(p.Path, "kapp-diff-"+name+".json")))
		}
	}

	return sections
}

// diffHelm compares the helm releases with helmfile. It does not depend on the diff.sh template.
func (p *Plugins) diffHelm(releases []string) []Section {
	if len(releases) == 0 {
		return nil
	}

	if err := p.helmfileRunner.Init(p.HelmPath); err != nil {
		reason := fmt.Sprintf("helmfile init failed: %v", err)

		return []Section{{Title: HelmGroup, Status: StatusFailed, Reason: reason}}
	}

	stdout, stderr, code, err := p.helmfileRunner.Diff()
	if err != nil {
		return []Section{{Title: HelmGroup, Status: StatusFailed, Reason: err.Error()}}
	}

	res := helmfile.ParseHelmDiff(stdout, stderr, code, releases)
	if res.Reason != "" {
		return []Section{{Title: HelmGroup, Status: StatusFailed, Reason: res.Reason}}
	}

	sections := make([]Section, 0, len(res.Releases))

	for _, rel := range res.Releases {
		title := helmTitle(rel.Name)

		if rel.NotCompared {
			sections = append(sections, Section{Title: title, Status: StatusFailed, Reason: rel.Reason})

			continue
		}

		section := Section{Title: title, Status: StatusCompared}

		for _, c := range rel.Changes {
			section.Rows = append(section.Rows, Row{
				Namespace: c.Namespace, Name: c.Name, Kind: c.Kind, API: c.API, Op: string(c.Change),
			})
		}

		sections = append(sections, section)
	}

	return sections
}
