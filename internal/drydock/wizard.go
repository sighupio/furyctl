// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package drydock serves a local web wizard that writes a furyctl.yaml step by step.
//
// A wizard is a YAML file of questions (steps and fields, see Wizard) plus a Go template
// that maps the answers to a furyctl.yaml. The questions stay declarative; the mapping
// lives in the template, the same language the distribution uses for its own templates.
package drydock

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/semver"
)

var ErrInvalidWizard = errors.New("invalid wizard")

// A nodeTable's config.domainFrom names a step and a field in it.
const domainFromParts = 2

// Text is a user-visible string in one or more languages, keyed by language code.
// In YAML it is either a plain string (English) or a {en: ..., it: ...} map.
type Text map[string]string

func (t *Text) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var s string
		if err := n.Decode(&s); err != nil {
			return fmt.Errorf("decoding text: %w", err)
		}

		*t = Text{"en": s}

		return nil
	}

	m := map[string]string{}
	if err := n.Decode(&m); err != nil {
		return fmt.Errorf("decoding text map: %w", err)
	}

	*t = m

	return nil
}

type Wizard struct {
	Kind           string `json:"kind"           yaml:"kind"`
	Versions       string `json:"versions"       yaml:"versions"`
	DefaultVersion string `json:"defaultVersion" yaml:"defaultVersion"`
	Template       string `json:"template"       yaml:"template"`
	Steps          []Step `json:"steps"          yaml:"steps"`
}

type Step struct {
	ID          string  `json:"id"                    yaml:"id"`
	Title       Text    `json:"title,omitempty"       yaml:"title"`
	Description Text    `json:"description,omitempty" yaml:"description"`
	Fields      []Field `json:"fields"                yaml:"fields"`
}

type Field struct {
	ID    string `json:"id"              yaml:"id"`
	Type  string `json:"type"            yaml:"type"`
	Label Text   `json:"label,omitempty" yaml:"label"`
	Help  Text   `json:"help,omitempty"  yaml:"help"`
	// Example is a realistic value, shown in the help tip next to the label.
	Example     string            `json:"example,omitempty"     yaml:"example"`
	Default     any               `json:"default,omitempty"     yaml:"default"`
	Required    bool              `json:"required,omitempty"    yaml:"required"`
	When        string            `json:"when,omitempty"        yaml:"when"`
	Suggest     string            `json:"suggest,omitempty"     yaml:"suggest"`
	Placeholder string            `json:"placeholder,omitempty" yaml:"placeholder"`
	Multiline   bool              `json:"multiline,omitempty"   yaml:"multiline"`
	Options     []any             `json:"options,omitempty"     yaml:"options"`
	Min         *float64          `json:"min,omitempty"         yaml:"min"`
	Max         *float64          `json:"max,omitempty"         yaml:"max"`
	Collapsed   bool              `json:"collapsed,omitempty"   yaml:"collapsed"`
	Item        *Field            `json:"item,omitempty"        yaml:"item"`    // List of scalars.
	Fields      []Field           `json:"fields,omitempty"      yaml:"fields"`  // Group, list of groups.
	Config      map[string]string `json:"config,omitempty"      yaml:"config"`  // Widget-specific.
	Target      string            `json:"target,omitempty"      yaml:"target"`  // Preset: the sibling list to fill.
	Presets     []Preset          `json:"presets,omitempty"     yaml:"presets"` // Preset: the entries offered.
}

// Preset is one known-good entry a `preset` field offers: a click adds it to the list named
// by the field's Target, another click takes it away. It exists for the settings an operator
// is expected to copy from documentation rather than invent.
type Preset struct {
	Label Text `json:"label"          yaml:"label"`
	Help  Text `json:"help,omitempty" yaml:"help"`
	Value any  `json:"value"          yaml:"value"`
	// Default puts the entry in the target list when the wizard opens, for the settings the
	// documentation prescribes rather than merely offers.
	Default bool `json:"default,omitempty" yaml:"default"`
}

func isFieldType(s string) bool {
	switch s {
	case "text", "number", "bool", "choice", "path", "cidr", "yaml", "keyValue",
		"list", "group", "nodeTable", "preset", "dataDisk":
		return true

	default:
		return false
	}
}

func isSuggest(s string) bool {
	switch s {
	case "", "env", "file", "path", "http":
		return true

	default:
		return false
	}
}

// Parse decodes a wizard file and checks its structure. Unknown keys are errors, so a typo
// in a wizard file fails the registry test instead of silently doing nothing.
func Parse(b []byte) (*Wizard, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)

	var w Wizard
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWizard, err)
	}

	if err := w.check(); err != nil {
		return nil, err
	}

	return &w, nil
}

func (w *Wizard) check() error {
	if w.Kind == "" {
		return fmt.Errorf("%w: kind is required", ErrInvalidWizard)
	}

	if _, err := semver.NewConstraint(w.Versions); err != nil {
		return fmt.Errorf("%w: versions %q: %w", ErrInvalidWizard, w.Versions, err)
	}

	if w.Template == "" {
		return fmt.Errorf("%w: template is required", ErrInvalidWizard)
	}

	seen := map[string]bool{}

	for _, s := range w.Steps {
		if s.ID == "" || seen[s.ID] {
			return fmt.Errorf("%w: step id %q missing or duplicated", ErrInvalidWizard, s.ID)
		}

		seen[s.ID] = true

		if err := checkFields(s.ID, s.Fields); err != nil {
			return err
		}
	}

	return nil
}

func checkFields(scope string, fields []Field) error {
	seen := map[string]bool{}
	lists := map[string]bool{}

	for i := range fields {
		if fields[i].Type == "list" {
			lists[fields[i].ID] = true
		}
	}

	for i := range fields {
		f := &fields[i]
		where := scope + "." + f.ID

		if f.ID == "" || seen[f.ID] {
			return fmt.Errorf("%w: field id %q missing or duplicated in %s", ErrInvalidWizard, f.ID, scope)
		}

		seen[f.ID] = true

		if !isFieldType(f.Type) {
			return fmt.Errorf("%w: %s: unknown type %q", ErrInvalidWizard, where, f.Type)
		}

		if !isSuggest(f.Suggest) {
			return fmt.Errorf("%w: %s: unknown suggest %q", ErrInvalidWizard, where, f.Suggest)
		}

		if _, err := ParseWhen(f.When); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrInvalidWizard, where, err)
		}

		if err := checkFieldShape(where, f); err != nil {
			return err
		}

		if f.Type == "preset" && !lists[f.Target] {
			return fmt.Errorf("%w: %s: target %q is not a list in the same step", ErrInvalidWizard, where, f.Target)
		}

		if len(f.Fields) > 0 {
			if err := checkFields(where, f.Fields); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkFieldShape(where string, f *Field) error {
	switch f.Type {
	case "choice":
		if len(f.Options) == 0 {
			return fmt.Errorf("%w: %s: choice needs options", ErrInvalidWizard, where)
		}

	case "list":
		if f.Item == nil && len(f.Fields) == 0 {
			return fmt.Errorf("%w: %s: list needs item or fields", ErrInvalidWizard, where)
		}

		if f.Item != nil {
			item := *f.Item
			if item.ID == "" {
				item.ID = "item"
			}

			return checkFields(where, []Field{item})
		}

	case "group":
		if len(f.Fields) == 0 {
			return fmt.Errorf("%w: %s: group needs fields", ErrInvalidWizard, where)
		}

	case "nodeTable":
		if err := checkNodeTableConfig(where, f.Config); err != nil {
			return err
		}

	case "keyValue":
		if f.Item != nil || len(f.Fields) > 0 {
			return fmt.Errorf("%w: %s: keyValue takes no item and no fields", ErrInvalidWizard, where)
		}

	case "preset":
		if f.Target == "" || len(f.Presets) == 0 {
			return fmt.Errorf("%w: %s: preset needs target and presets", ErrInvalidWizard, where)
		}

		for i, p := range f.Presets {
			if len(p.Label) == 0 || p.Value == nil {
				return fmt.Errorf("%w: %s: preset %d needs label and value", ErrInvalidWizard, where, i)
			}
		}

	default:
	}

	return nil
}

// checkNodeTableConfig keeps the widget's contract honest: a typo in a step name or in the list of
// columns would otherwise show up as an empty table rather than as a broken wizard.
func checkNodeTableConfig(where string, config map[string]string) error {
	// Only these two are dereferenced without a guard: the widget reads the roles out of the
	// topology and the node defaults out of its step.
	for _, key := range []string{"topologyStep", "defaultsStep"} {
		if config[key] == "" {
			return fmt.Errorf("%w: %s: nodeTable needs config.%s", ErrInvalidWizard, where, key)
		}
	}

	// `domainFrom` is a step.field path and is optional: without it the generated hostnames stay
	// short, which is what a provider that composes the fully qualified name itself expects.
	if from := config["domainFrom"]; from != "" && len(strings.Split(from, ".")) != domainFromParts {
		return fmt.Errorf("%w: %s: domainFrom must be a step.field path, got %q", ErrInvalidWizard, where, from)
	}

	for column := range strings.SplitSeq(config["columns"], ",") {
		switch strings.TrimSpace(column) {
		case "", "hostname", "macAddress", "ip":

		default:
			return fmt.Errorf("%w: %s: unknown column %q", ErrInvalidWizard, where, column)
		}
	}

	return nil
}
