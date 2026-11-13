// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

const crdKind = "CustomResourceDefinition"

// object holds the fields of a kapp inspect --raw document that the grace report reads.
type object struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   objectMeta `yaml:"metadata"`
	// Set only for CustomResourceDefinitions: the group and kind that the CRD serves.
	served crdServed
}

type objectMeta struct {
	Name              string   `yaml:"name"`
	Namespace         string   `yaml:"namespace"`
	DeletionTimestamp string   `yaml:"deletionTimestamp"`
	Finalizers        []string `yaml:"finalizers"`
}

type crd struct {
	Spec crdServed `yaml:"spec"`
}

type crdServed struct {
	Group string   `yaml:"group"`
	Names crdNames `yaml:"names"`
}

type crdNames struct {
	Kind string `yaml:"kind"`
}

func (o object) ref() string {
	if o.Metadata.Namespace == "" {
		return o.Metadata.Name
	}

	return o.Metadata.Namespace + "/" + o.Metadata.Name
}

// stuck reports an object that the API server keeps only for its finalizers.
func (o object) stuck() bool {
	return o.Metadata.DeletionTimestamp != ""
}

func (o object) group() string {
	group, _, found := strings.Cut(o.APIVersion, "/")
	if !found {
		return ""
	}

	return group
}

// parseObjects reads kapp inspect --raw: "---" before each object, then "Succeeded" (kapp 0.66.1 cmd/kapp/kapp.go).
// It reads helm manifests too, whose comment-only documents it skips.
func parseObjects(raw string) ([]object, error) {
	start := strings.Index(raw, "---\n")
	if start < 0 {
		return nil, nil
	}

	dec := yaml.NewDecoder(strings.NewReader(strings.TrimSuffix(strings.TrimRight(raw[start:], "\n"), "\nSucceeded")))

	var objs []object

	for {
		var node yaml.Node

		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			return objs, nil
		}

		if err != nil {
			return nil, fmt.Errorf("error reading the objects: %w", err)
		}

		var o object
		if err := node.Decode(&o); err != nil {
			return nil, fmt.Errorf("error reading the objects: %w", err)
		}

		if o.Kind == crdKind {
			var c crd
			if err := node.Decode(&c); err != nil {
				return nil, fmt.Errorf("error reading the objects: %w", err)
			}

			o.served = c.Spec
		}

		if o.Kind != "" {
			objs = append(objs, o)
		}
	}
}

func objectSelector(o object) selector {
	s := selector{Kinds: []string{o.Kind}, Names: []string{o.Metadata.Name}}
	if o.Metadata.Namespace != "" {
		s.Namespaces = []string{o.Metadata.Namespace}
	}

	return s
}
