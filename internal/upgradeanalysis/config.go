// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	"github.com/sighupio/furyctl/internal/schemadiff"
)

// Severity ranks a finding by what it demands of whoever runs the upgrade.
const (
	// SeverityBlocker marks a change that will make the upgrade fail or silently misbehave
	// unless the configuration is edited first.
	SeverityBlocker = "blocker"
	// SeverityAction marks something that needs a decision but does not stop the upgrade.
	SeverityAction = "action"
)

// Finding is one thing about the stored configuration that the upgrade requires attention for.
type Finding struct {
	Severity string `json:"severity"       yaml:"severity"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Message  string `json:"message"        yaml:"message"`
}

// configFindings compares the schema of the current version with the schema of the target
// and keeps only what actually touches this cluster's configuration: a property that
// disappears while the cluster sets it, and a property that becomes required while the
// cluster does not set it.
//
// Schema definitions that describe list items, such as Spec.Kubernetes.Nodes.Node.Host,
// cannot be located in the configuration by name, so changes confined to those are not
// reported. Everything they would have caught still has to be read from the release notes.
func configFindings(
	current, target Distribution,
	kind string,
	cfg map[string]any,
	targetVersion string,
) ([]Finding, error) {
	currentSchema, err := readSchema(current.Path, kind)
	if err != nil {
		return nil, err
	}

	targetSchema, err := readSchema(target.Path, kind)
	if err != nil {
		return nil, err
	}

	changes, err := schemadiff.Compare(currentSchema, targetSchema)
	if err != nil {
		return nil, fmt.Errorf("cannot compare the configuration schemas: %w", err)
	}

	var findings []Finding

	for _, change := range changes {
		if finding, relevant := classify(change, changes, cfg, targetVersion); relevant {
			findings = append(findings, finding)
		}
	}

	return findings, nil
}

// classify decides whether a schema change matters for this cluster's configuration.
func classify(
	change schemadiff.Change,
	all []schemadiff.Change,
	cfg map[string]any,
	targetVersion string,
) (Finding, bool) {
	path := configPath(change.Definition)
	if path == nil {
		return Finding{}, false
	}

	full := append(slices.Clone(path), change.Property)
	dotted := strings.Join(full, ".")

	switch change.Kind {
	case schemadiff.PropertyRemoved:
		// Only a problem when the cluster actually sets the property.
		if !isSet(cfg, full) {
			return Finding{}, false
		}

		message := fmt.Sprintf("%s is set but no longer exists in %s", dotted, targetVersion)
		if replacement, renamed := renameHint(change, all); renamed {
			message += fmt.Sprintf("; %s.%s was added in its place, likely a rename",
				strings.Join(path, "."), replacement)
		}

		return Finding{Severity: SeverityBlocker, Path: dotted, Message: message}, true

	case schemadiff.RequiredAdded:
		// Only a problem when the parent object exists and the key is missing from it.
		if !isSet(cfg, path) {
			return Finding{}, false
		}

		if isSet(cfg, full) {
			return Finding{}, false
		}

		return Finding{
			Severity: SeverityBlocker,
			Path:     dotted,
			Message:  fmt.Sprintf("%s becomes required in %s and is not set", dotted, targetVersion),
		}, true

	// Everything else is a relaxed requirement or a new optional property, and neither
	// can break a configuration that already exists.
	default:
		return Finding{}, false
	}
}

// renameHint looks for a property added to the same definition that lost one, which is how
// a rename shows up in a schema diff.
func renameHint(removed schemadiff.Change, all []schemadiff.Change) (string, bool) {
	for _, change := range all {
		if change.Kind == schemadiff.PropertyAdded && change.Definition == removed.Definition {
			return change.Property, true
		}
	}

	return "", false
}

// configPath turns a schema definition name into the path of the matching configuration
// key. Schema definitions are dotted paths in upper camel case, so lowering the first
// letter of every segment gives the configuration path. It returns nil for the root
// definition and for anything that is not shaped like a path.
func configPath(definition string) []string {
	if definition == schemadiff.RootDefinition || definition == "" {
		return nil
	}

	segments := strings.Split(definition, ".")
	path := make([]string, 0, len(segments))

	for _, segment := range segments {
		if segment == "" {
			return nil
		}

		runes := []rune(segment)
		runes[0] = unicode.ToLower(runes[0])
		path = append(path, string(runes))
	}

	return path
}

// isSet reports whether a path exists in the rendered configuration.
func isSet(cfg map[string]any, path []string) bool {
	var current any = cfg

	for _, segment := range path {
		asMap, isMap := current.(map[string]any)
		if !isMap {
			return false
		}

		value, present := asMap[segment]
		if !present {
			return false
		}

		current = value
	}

	return true
}

// validationFindings runs the stored configuration against the schema of the target
// version. It complements the schema diff: validation catches what the schema enforces,
// the diff catches what it fails to enforce.
func validationFindings(target Distribution, kind string, cfg map[string]any, targetVersion string) []Finding {
	path, err := schemaPath(target.Path, kind)
	if err != nil {
		return nil
	}

	schema, err := santhosh.LoadSchema(path)
	if err != nil {
		return nil
	}

	if err := schema.Validate(cfg); err != nil {
		return []Finding{{
			Severity: SeverityAction,
			Message: fmt.Sprintf(
				"the stored configuration does not validate against the %s schema: %v",
				targetVersion, err,
			),
		}}
	}

	return nil
}

func readSchema(repoPath, kind string) ([]byte, error) {
	path, err := schemaPath(repoPath, kind)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the configuration schema: %w", err)
	}

	return raw, nil
}

// schemaPath locates the public schema of a kind inside a downloaded distribution.
func schemaPath(repoPath, kind string) (string, error) {
	path, err := distribution.GetPublicSchemaPath(repoPath, config.Furyctl{
		APIVersion: "kfd.sighup.io/v1alpha2",
		Kind:       kind,
	})
	if err != nil {
		return "", fmt.Errorf("cannot locate the configuration schema: %w", err)
	}

	return path, nil
}
