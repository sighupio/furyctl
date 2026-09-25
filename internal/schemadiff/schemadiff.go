// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package schemadiff compares two versions of a furyctl JSON schema and reports the
// structural changes that can break an existing configuration file: properties that
// disappear, and properties that become required.
//
// This is deliberately separate from schema validation. Validating a configuration
// against the target schema catches what the schema enforces; this package catches what it
// should enforce but does not. A schema that nests `additionalProperties: false` inside
// `properties` rather than beside it, for instance, keeps accepting a key that has been
// removed, so the configuration still validates while the key no longer has any effect.
package schemadiff

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// ChangeKind is what happened to a property between two schema versions.
type ChangeKind string

const (
	// PropertyAdded marks a property that the target schema accepts and the current one does not.
	PropertyAdded ChangeKind = "property-added"
	// PropertyRemoved marks a property the target schema no longer accepts.
	PropertyRemoved ChangeKind = "property-removed"
	// RequiredAdded marks a property that the target schema requires and the current one does not.
	RequiredAdded ChangeKind = "required-added"
	// RequiredRemoved marks a property that is no longer mandatory.
	RequiredRemoved ChangeKind = "required-removed"

	// RootDefinition is the name reported for changes to the top level of the schema,
	// which lives outside "$defs".
	RootDefinition = "(root)"
)

// Change is one structural difference between two schema versions.
type Change struct {
	// Definition is the "$defs" entry the change belongs to. In furyctl schemas these are
	// dotted paths, such as Spec.Distribution.Modules.Logging.CustomOutputs.
	Definition string     `json:"definition" yaml:"definition"`
	Property   string     `json:"property"   yaml:"property"`
	Kind       ChangeKind `json:"kind"       yaml:"kind"`
}

// Compare reports how the target schema differs from the current one.
//
// Only definitions present in both schemas are compared. A definition that only exists in
// the target describes configuration that could not have been written before, and one that
// only exists in the current schema is reported through its parent, as the property that
// referenced it disappearing.
func Compare(current, target []byte) ([]Change, error) {
	currentDefs, err := definitions(current)
	if err != nil {
		return nil, fmt.Errorf("cannot read the current schema: %w", err)
	}

	targetDefs, err := definitions(target)
	if err != nil {
		return nil, fmt.Errorf("cannot read the target schema: %w", err)
	}

	var changes []Change

	for name, currentDef := range currentDefs {
		targetDef, common := targetDefs[name]
		if !common {
			continue
		}

		changes = append(changes, compareDefinition(name, currentDef, targetDef)...)
	}

	slices.SortFunc(changes, func(a, b Change) int {
		if c := strings.Compare(a.Definition, b.Definition); c != 0 {
			return c
		}

		if c := strings.Compare(a.Property, b.Property); c != 0 {
			return c
		}

		return strings.Compare(string(a.Kind), string(b.Kind))
	})

	return changes, nil
}

// compareDefinition reports the property and requirement changes of a single definition.
func compareDefinition(name string, current, target map[string]any) []Change {
	var changes []Change

	currentProps, targetProps := properties(current), properties(target)

	for prop := range currentProps {
		if _, kept := targetProps[prop]; !kept {
			changes = append(changes, Change{Definition: name, Property: prop, Kind: PropertyRemoved})
		}
	}

	for prop := range targetProps {
		if _, existed := currentProps[prop]; !existed {
			changes = append(changes, Change{Definition: name, Property: prop, Kind: PropertyAdded})
		}
	}

	currentRequired, targetRequired := required(current), required(target)

	for _, prop := range targetRequired {
		if !slices.Contains(currentRequired, prop) {
			changes = append(changes, Change{Definition: name, Property: prop, Kind: RequiredAdded})
		}
	}

	for _, prop := range currentRequired {
		if !slices.Contains(targetRequired, prop) {
			changes = append(changes, Change{Definition: name, Property: prop, Kind: RequiredRemoved})
		}
	}

	return changes
}

// definitions returns every comparable object of a schema, keyed by name: the "$defs"
// entries plus the root under RootDefinition.
//
// Objects nested inline inside a definition are collected too, under a dotted name such as
// Spec.Kubernetes.Advanced.kubeProxy. Without this, a change confined to an inline object
// would be invisible, since its parent keeps listing the same property.
func definitions(raw []byte) (map[string]map[string]any, error) {
	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	defs := map[string]map[string]any{}
	collect(RootDefinition, root, defs)

	rawDefs, ok := root["$defs"].(map[string]any)
	if !ok {
		return defs, nil
	}

	for name, def := range rawDefs {
		if typed, isObject := def.(map[string]any); isObject {
			collect(name, typed, defs)
		}
	}

	return defs, nil
}

// collect records a definition and walks into the objects declared inline within it.
func collect(name string, def map[string]any, out map[string]map[string]any) {
	out[name] = def

	for prop, value := range properties(def) {
		child, isObject := value.(map[string]any)
		if !isObject {
			continue
		}

		// Only objects that declare a shape of their own are worth comparing; a property
		// that merely points at a "$ref" is compared through the definition it names.
		if !hasShape(child) {
			continue
		}

		collect(name+"."+prop, child, out)
	}
}

// hasShape reports whether a schema object declares properties or requirements of its own.
func hasShape(def map[string]any) bool {
	if _, ok := def["properties"]; ok {
		return true
	}

	_, ok := def["required"]

	return ok
}

func properties(def map[string]any) map[string]any {
	props, ok := def["properties"].(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return props
}

func required(def map[string]any) []string {
	raw, ok := def["required"].([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(raw))

	for _, item := range raw {
		if name, isString := item.(string); isString {
			out = append(out, name)
		}
	}

	return out
}
