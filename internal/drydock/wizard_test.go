// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package drydock

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalWizard = `
kind: Immutable
versions: ">= 1.35.1, < 1.36.0"
defaultVersion: v1.35.1
template: immutable-v1.35.yaml.tpl
steps:
  - id: cluster
    title: Cluster
    fields:
      - id: name
        type: text
        label: {en: Cluster name, it: Nome del cluster}
        required: true
      - id: lbMode
        type: choice
        options: [none, dedicated]
        default: dedicated
      - id: lbCount
        type: choice
        options: [1, 2]
        when: lbMode == dedicated
      - id: proxy
        type: group
        collapsed: true
        fields:
          - id: http
            type: text
      - id: nameservers
        type: list
        item:
          type: text
      - id: disks
        type: list
        fields:
          - id: device
            type: text
  - id: nodes
    fields:
      - id: nodes
        type: nodeTable
        config: {topologyStep: cluster, defaultsStep: cluster}
`

func TestParseWizard(t *testing.T) {
	t.Parallel()

	w, err := Parse([]byte(minimalWizard))
	require.NoError(t, err)

	assert.Equal(t, "Immutable", w.Kind)
	assert.Equal(t, "v1.35.1", w.DefaultVersion)
	assert.Len(t, w.Steps, 2)
	assert.Equal(t, Text{"en": "Cluster"}, w.Steps[0].Title)
	assert.Equal(t, Text{"en": "Cluster name", "it": "Nome del cluster"}, w.Steps[0].Fields[0].Label)
	assert.Equal(t, "text", w.Steps[0].Fields[4].Item.Type)
	assert.Equal(t, "device", w.Steps[0].Fields[5].Fields[0].ID)

	// Text marshals to JSON as a plain object, so the UI reads label.en / label.it.
	b, err := json.Marshal(w.Steps[0].Fields[0].Label)
	require.NoError(t, err)
	assert.JSONEq(t, `{"en":"Cluster name","it":"Nome del cluster"}`, string(b))
}

func TestParseWizardRejects(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing kind":       "versions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: []}]",
		"bad versions":       "kind: K\nversions: 'banana'\ntemplate: x\nsteps: [{id: a, fields: []}]",
		"missing template":   "kind: K\nversions: '>= 1'\nsteps: [{id: a, fields: []}]",
		"duplicate step":     "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: []}, {id: a, fields: []}]",
		"duplicate field":    "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: text}, {id: f, type: text}]}]",
		"unknown type":       "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: banana}]}]",
		"bad when":           "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: text, when: 'a =='}]}]",
		"choice w/o options": "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: choice}]}]",
		"list w/o item":      "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: list}]}]",
		"group w/o fields":   "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: group}]}]",
		"unknown key":        "kind: K\nversions: '>= 1'\ntemplate: x\nbanana: 1\nsteps: [{id: a, fields: []}]",
		"bad suggest":        "kind: K\nversions: '>= 1'\ntemplate: x\nsteps: [{id: a, fields: [{id: f, type: text, suggest: ftp}]}]",
	}

	for name, src := range cases {
		_, err := Parse([]byte(src))
		require.ErrorIs(t, err, ErrInvalidWizard, name)
	}
}
