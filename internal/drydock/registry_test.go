// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package drydock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadEmbedded(t *testing.T) {
	t.Parallel()

	reg, err := LoadEmbedded()
	require.NoError(t, err)

	list := reg.List()
	require.Len(t, list, 1)
	assert.Equal(t, WizardInfo{Kind: "Immutable", Range: ">= 1.35.1, < 1.36.0", DefaultVersion: "v1.35.1", Versions: []string{}}, list[0])

	w, tpl, err := reg.Find("Immutable", "v1.35.1")
	require.NoError(t, err)
	assert.Equal(t, "Immutable", w.Kind)
	assert.NotEmpty(t, tpl)

	// Every step of the Immutable wizard the spec lists, in order.
	ids := make([]string, 0, len(w.Steps))
	for _, s := range w.Steps {
		ids = append(ids, s.ID)
	}

	assert.Equal(t, []string{"cluster", "topology", "nodeDefaults", "nodes", "kubernetes", "modules"}, ids)

	_, _, err = reg.Find("Immutable", "v1.34.0")
	require.ErrorIs(t, err, ErrNoWizard)

	_, _, err = reg.Find("OnPremises", "v1.35.1")
	require.ErrorIs(t, err, ErrNoWizard)

	_, _, err = reg.Find("Immutable", "banana")
	require.Error(t, err)
}

// Every field the user types into carries an example, so the (?) tip is never empty.
// The file examples are transcribed from the documentation, so at least the one that is YAML has to
// still parse as the object it claims to be.
func TestEncryptionFileExampleIsAnEncryptionConfiguration(t *testing.T) {
	t.Parallel()

	reg, err := LoadEmbedded()
	require.NoError(t, err)

	w, _, err := reg.Find("Immutable", "v1.35.1")
	require.NoError(t, err)

	var found string

	for _, s := range w.Steps {
		for _, f := range s.Fields {
			if f.ID == "encryptionConfig" {
				found = f.FileExample
			}
		}
	}

	require.NotEmpty(t, found, "the encryption configuration field carries no file example")

	var manifest struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Resources  []struct {
			Resources []string `yaml:"resources"`
		} `yaml:"resources"`
	}

	require.NoError(t, yaml.Unmarshal([]byte(found), &manifest))
	assert.Equal(t, "apiserver.config.k8s.io/v1", manifest.APIVersion)
	assert.Equal(t, "EncryptionConfiguration", manifest.Kind)
	require.Len(t, manifest.Resources, 1)
	assert.Equal(t, []string{"secrets"}, manifest.Resources[0].Resources)
}

func TestEmbeddedWizardsHaveExamples(t *testing.T) {
	t.Parallel()

	reg, err := LoadEmbedded()
	require.NoError(t, err)

	w, _, err := reg.Find("Immutable", "v1.35.1")
	require.NoError(t, err)

	var missing []string

	var walk func(scope string, fields []Field)

	walk = func(scope string, fields []Field) {
		for _, f := range fields {
			where := scope + "." + f.ID

			switch {
			// Containers carry no value of their own; their fields are the ones that need an example.
			case f.Type == "group" || f.Type == "table" || f.Type == "nodeTable" || f.Type == "dataDisk" ||
				(f.Type == "list" && f.Item == nil):
				walk(where, f.Fields)

			case f.Type == "bool" || f.Type == "choice" || f.Type == "preset":
				// Self-explanatory controls.

			case f.Example == "":
				missing = append(missing, where)
			}
		}
	}

	for _, s := range w.Steps {
		walk(s.ID, s.Fields)
	}

	assert.Empty(t, missing)
}
