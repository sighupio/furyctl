// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package drydock

import (
	"embed"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
)

var (
	//go:embed wizards
	wizardsFS embed.FS

	ErrNoWizard = errors.New("no wizard for this kind and version")
)

type WizardInfo struct {
	Kind           string   `json:"kind"`
	Range          string   `json:"range"`
	DefaultVersion string   `json:"defaultVersion"`
	Versions       []string `json:"versions"` // Concrete releases in Range, filled by the server.
}

type Registry struct {
	wizards   []*Wizard
	templates map[string]string
}

// LoadEmbedded parses every wizards/*.yaml and reads the template each one names.
// A broken wizard file makes furyctl drydock fail at start, and the registry test fail in CI.
func LoadEmbedded() (*Registry, error) {
	entries, err := wizardsFS.ReadDir("wizards")
	if err != nil {
		return nil, fmt.Errorf("reading embedded wizards: %w", err)
	}

	reg := &Registry{templates: map[string]string{}}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}

		b, err := wizardsFS.ReadFile(path.Join("wizards", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}

		w, err := Parse(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}

		tpl, err := wizardsFS.ReadFile(path.Join("wizards", w.Template))
		if err != nil {
			return nil, fmt.Errorf("%s: template %q: %w", e.Name(), w.Template, err)
		}

		reg.wizards = append(reg.wizards, w)
		reg.templates[w.Template] = string(tpl)
	}

	return reg, nil
}

func (r *Registry) List() []WizardInfo {
	out := make([]WizardInfo, 0, len(r.wizards))

	for _, w := range r.wizards {
		out = append(out, WizardInfo{Kind: w.Kind, Range: w.Versions, DefaultVersion: w.DefaultVersion, Versions: []string{}})
	}

	return out
}

// InRange reports whether version falls in the wizard's range.
func (w *Wizard) InRange(version string) bool {
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	c, err := semver.NewConstraint(w.Versions)

	return err == nil && c.Check(v)
}

// Find returns the wizard for kind whose version range contains version, and its template.
func (r *Registry) Find(kind, version string) (*Wizard, string, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, "", fmt.Errorf("parsing version %q: %w", version, err)
	}

	for _, w := range r.wizards {
		if w.Kind != kind {
			continue
		}

		c, err := semver.NewConstraint(w.Versions)
		if err != nil {
			return nil, "", fmt.Errorf("wizard %s versions %q: %w", w.Kind, w.Versions, err)
		}

		if c.Check(v) {
			return w, r.templates[w.Template], nil
		}
	}

	return nil, "", fmt.Errorf("%w: %s %s", ErrNoWizard, kind, version)
}
