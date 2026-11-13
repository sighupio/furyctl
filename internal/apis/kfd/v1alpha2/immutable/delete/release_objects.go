// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"slices"
	"strings"
)

// releaseObjects returns the release objects served by a CRD in the cluster, and the CRDs the release ships.
func (p *Plugins) releaseObjects(r release) ([]object, []object, error) {
	objs, err := p.releaseManifests(r)
	if err != nil {
		return nil, nil, err
	}

	served, err := p.servedKinds()
	if err != nil {
		return nil, nil, err
	}

	crs, crds := splitCustom(objs, served, r.namespace)

	return crs, crds, nil
}

// releaseManifests returns the objects that the release installed, then the CRDs that its chart ships.
func (p *Plugins) releaseManifests(r release) ([]object, error) {
	manifest, err := p.runners.Helm.GetManifest(r.name, r.namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading helm release %s: %w", r.name, err)
	}

	objs, err := parseObjects("---\n" + manifest)
	if err != nil {
		return nil, fmt.Errorf("error reading helm release %s: %w", r.name, err)
	}

	chartCRDs, err := p.runners.Helm.ShowCRDs(r.chart, r.version)
	if err != nil {
		return nil, fmt.Errorf("error reading the CRDs of helm release %s: %w", r.name, err)
	}

	shipped, err := parseObjects("---\n" + chartCRDs)
	if err != nil {
		return nil, fmt.Errorf("error reading the CRDs of helm release %s: %w", r.name, err)
	}

	return append(objs, shipped...), nil
}

// splitCustom returns the objects of a kind in served, by group/kind, and the distinct CRDs among objs.
func splitCustom(objs []object, served map[string]string, namespace string) ([]object, []object) {
	var crs, crds []object

	for _, o := range objs {
		if o.Kind != crdKind {
			scope, isCR := served[o.group()+"/"+o.Kind]
			if isCR {
				// Helm installs namespaced objects without a namespace in the release namespace.
				if scope == "Namespaced" && o.Metadata.Namespace == "" {
					o.Metadata.Namespace = namespace
				}

				crs = append(crs, o)
			}

			continue
		}

		if !slices.ContainsFunc(crds, func(c object) bool { return c.Metadata.Name == o.Metadata.Name }) {
			crds = append(crds, o)
		}
	}

	return crs, crds
}

// servedKinds returns the scope of each group/kind that a CRD in the cluster serves.
func (p *Plugins) servedKinds() (map[string]string, error) {
	out, err := p.runners.Kubectl.Get(false, "all", "customresourcedefinitions",
		"-o", `jsonpath={range .items[*]}{.spec.group}/{.spec.names.kind}={.spec.scope}{"\n"}{end}`)
	if err != nil {
		return nil, fmt.Errorf("error listing the CRDs of the cluster: %w", err)
	}

	served := map[string]string{}

	for line := range strings.FieldsSeq(out) {
		kind, scope, _ := strings.Cut(line, "=")
		served[kind] = scope
	}

	return served, nil
}
