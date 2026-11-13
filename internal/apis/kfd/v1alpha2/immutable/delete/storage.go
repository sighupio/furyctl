// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	kappAppLabel          = "kapp.k14s.io/app"
	helmReleaseAnnotation = "meta.helm.sh/release-name"
)

// ErrStorageInUse stops --phase plugins while a StorageClass of a plugin still serves PVCs of other workloads.
var ErrStorageInUse = errors.New("PVCs outside the plugins are still bound to a StorageClass of the plugins")

type kubeList struct {
	Items []kubeItem `yaml:"items"`
}

type kubeItem struct {
	Metadata kubeMeta `yaml:"metadata"`
	Spec     pvcSpec  `yaml:"spec"`
}

type kubeMeta struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type pvcSpec struct {
	StorageClassName string `yaml:"storageClassName"`
}

// owns reports whether the object carries the kapp app label or the helm release annotation of the plugin.
func (p Plugin) owns(m kubeMeta) bool {
	if p.Helm {
		return m.Annotations[helmReleaseAnnotation] == p.Name
	}

	return m.Labels[kappAppLabel] == p.app()
}

// StorageProviders returns the plugins that own a StorageClass in the cluster: apply runs the distribution again
// after them, so their delete goes after the distribution.
func (p *Plugins) StorageProviders(plugins []Plugin) ([]Plugin, error) {
	classes, err := p.listCluster("storageclasses")
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(slices.Clone(plugins), func(plugin Plugin) bool {
		return len(storageClassesOf(plugin, classes)) == 0
	}), nil
}

// CheckStorageInUse returns ErrStorageInUse naming the PVCs that are bound to a StorageClass of the plugins and do not
// belong to the plugin: deleting the provisioner first leaves their volumes behind.
func (p *Plugins) CheckStorageInUse(plugins []Plugin) error {
	classes, err := p.listCluster("storageclasses")
	if err != nil {
		return err
	}

	pvcs, err := p.listCluster("persistentvolumeclaims")
	if err != nil {
		return err
	}

	var inUse []string

	for _, plugin := range plugins {
		owned := storageClassesOf(plugin, classes)

		for _, pvc := range pvcs {
			if slices.Contains(owned, pvc.Spec.StorageClassName) && !plugin.ownsPVC(pvc.Metadata) {
				inUse = append(inUse, pvc.Metadata.Namespace+"/"+pvc.Metadata.Name)
			}
		}
	}

	if len(inUse) > 0 {
		return fmt.Errorf("%w: %s; run furyctl delete cluster --phase distribution first",
			ErrStorageInUse, strings.Join(inUse, ", "))
	}

	return nil
}

// ownsPVC reports whether a PVC is the plugin's own: kapp labels the PVCs of its StatefulSets, helm ones live in the
// release namespace.
func (p Plugin) ownsPVC(m kubeMeta) bool {
	return p.owns(m) || p.Helm && p.release.namespace != "" && m.Namespace == p.release.namespace
}

func storageClassesOf(plugin Plugin, classes []kubeItem) []string {
	var names []string

	for _, c := range classes {
		if plugin.owns(c.Metadata) {
			names = append(names, c.Metadata.Name)
		}
	}

	return names
}

func (p *Plugins) listCluster(resource string) ([]kubeItem, error) {
	out, err := p.runners.Kubectl.Get(false, "all", resource, "-o", "yaml")
	if err != nil {
		return nil, fmt.Errorf("error listing the %s of the cluster: %w", resource, err)
	}

	var list kubeList
	if err := yaml.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("error reading the %s of the cluster: %w", resource, err)
	}

	return list.Items, nil
}
