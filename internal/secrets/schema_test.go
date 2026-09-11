// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package secrets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	distroconf "github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/secrets"
)

// The distribution builds the public schemas with these shapes: a reference to a definition of the
// same file, a reference to a file of its own, a schema that holds the fields of other schemas, and
// a field that only the clusters that a condition matches have. The Pomerium secrets live in a file
// of their own, thus that shape carries four of the secrets that furyctl creates.
//
// Each object rejects the names that it does not hold, as each public schema of the distribution
// does more than a hundred times. Without that, a schema takes a field with any name and Has has
// nothing to answer no about.
const (
	schemaWithEachShape = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "spec": { "$ref": "#/$defs/Spec" }
  },
  "$defs": {
    "Spec": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "plain": {
          "type": "object",
          "additionalProperties": false,
          "properties": { "field": { "type": "string" } }
        },
        "pomerium": { "$ref": "./pomerium.json" },
        "labels": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        },
        "distribution": { "type": "object" },
        "shared": true,
        "conditional": true
      },
      "allOf": [
        {
          "properties": {
            "shared": {
              "type": "object",
              "additionalProperties": false,
              "properties": { "field": { "type": "string" } }
            }
          }
        }
      ],
      "if": { "properties": { "plain": { "const": "yes" } } },
      "then": {
        "properties": {
          "conditional": {
            "type": "object",
            "additionalProperties": false,
            "properties": { "field": { "type": "string" } }
          }
        }
      }
    }
  }
}`

	pomeriumSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "secrets": {
      "type": "object",
      "additionalProperties": false,
      "properties": { "SIGNING_KEY": { "type": "string" } }
    }
  }
}`
)

// TestSchemaHas covers each shape that the distribution builds the public schemas with. A field
// that furyctl cannot find is a secret that furyctl does not create. A shape that this answer
// cannot follow thus stops the cluster: it does not get the secrets that it needs.
//
// The answer comes from the schema itself, thus it is the answer of `furyctl validate config`. A
// name that the schema holds and a value of the wrong type is thus a yes: the name is one that the
// cluster can hold, and furyctl writes a value of the right type to it.
func TestSchemaHas(t *testing.T) {
	t.Parallel()

	schema := loadTestSchema(t)

	tests := []struct {
		desc string
		path string
		has  bool
	}{
		{"the field that LoadSchema reads as its anchor", "spec.distribution", true},
		{"a field of a definition", "spec.plain.field", true},
		{"a field of a shared schema that the schema has not", "spec.shared.bogus", false},
		{"a field of a schema of its own", "spec.pomerium.secrets.SIGNING_KEY", true},
		{"a field of a shared schema", "spec.shared.field", true},
		{"a field of an object with free-form fields", "spec.labels.anyName", true},
		{"a field that the schema holds with another type", "spec.plain.field.deeper", true},
		{"a field that the schema has not", "spec.plain.missing", false},
		{"a field under a field that the schema has not", "spec.missing.field", false},
		{"a field of a schema of its own that is not there", "spec.pomerium.secrets.COOKIE_SECRET", false},
		{"a field of an object that holds no such name", "spec.pomerium.bogus", false},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.has, schema.Has(test.path), test.path)
		})
	}
}

// TestSchemaHasWithNoSchema keeps the answer of a cluster whose schema furyctl cannot read. A
// secret too many is a file that nothing reads, while a secret too few keeps a cluster from
// starting: the answer is thus yes to every field.
func TestSchemaHasWithNoSchema(t *testing.T) {
	t.Parallel()

	var schema *secrets.Schema

	assert.True(t, schema.Has("spec.anything.at.all"))
}

func TestKeep(t *testing.T) {
	t.Parallel()

	components := []secrets.Component{
		{
			Name: "kept",
			Secrets: []secrets.Secret{
				{Name: "field", Path: "spec.plain.field"},
				{Name: "missing", Path: "spec.plain.missing"},
			},
		},
		{
			Name:    "dropped",
			Secrets: []secrets.Secret{{Name: "missing", Path: "spec.missing.field"}},
		},
	}

	kept, dropped := secrets.Keep(components, loadTestSchema(t))

	require.Len(t, kept, 1)
	assert.Equal(t, "kept", kept[0].Name)
	assert.Equal(t, []string{"dropped"}, dropped)

	// The component keeps only the secret that the cluster has a field for.
	require.Len(t, kept[0].Secrets, 1)
	assert.Equal(t, "field", kept[0].Secrets[0].Name)

	// Keep reads the catalog and never changes it: another run gives the same components.
	assert.Len(t, components[0].Secrets, 2)
}

// TestEncryptionReadsTheSchema keeps the encryption at rest out of the clusters that have no field
// for it. Only the schema of the kind and of the version of the distribution knows: the field
// arrived in v1.26.6, and the kinds that run their own control plane are the kinds that have it.
func TestEncryptionReadsTheSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte("kind: OnPremises"), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	// The schema of the shapes holds no encryption field, thus furyctl asks nothing about it.
	_, supported := secrets.Encryption(cfg, loadTestSchema(t))
	assert.False(t, supported)
}

// TestEncryptionLeavesAFieldThatTheClusterCannotHold keeps the run alive for a configuration that
// carries an encryption field to a cluster with no place for it. The schema gives the answer, thus
// furyctl never reads that field: a read that it cannot make would stop the run over a field that
// the cluster does not hold.
func TestEncryptionLeavesAFieldThatTheClusterCannotHold(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	config := `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: "{file://./gone.yaml}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	_, supported := secrets.Encryption(cfg, loadTestSchema(t))

	assert.False(t, supported)
	assert.Empty(t, cfg.Unresolved(), "furyctl named a field that the cluster cannot hold")
}

// loadTestSchema writes the schema of the shapes and the schema of Pomerium next to each other, as
// the distribution keeps them, and reads the first one.
func loadTestSchema(t *testing.T) *secrets.Schema {
	t.Helper()

	folder := filepath.Join(t.TempDir(), "schemas", "public")
	require.NoError(t, os.MkdirAll(folder, 0o700))

	files := map[string]string{
		"onpremises-kfd-v1alpha2.json": schemaWithEachShape,
		"pomerium.json":                pomeriumSchema,
	}

	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(folder, name), []byte(content), 0o600))
	}

	repoPath := filepath.Dir(filepath.Dir(folder))

	schema, err := secrets.LoadSchema(repoPath, distroconf.Furyctl{
		APIVersion: "kfd.sighup.io/v1alpha2",
		Kind:       "OnPremises",
	})
	require.NoError(t, err)

	return schema
}

// TestLoadSchemaRejectsASchemaWithNoShape covers the schema that fails the quiet way. A file that is
// valid JSON and turns each field away answers no to every path, thus Keep drops every component
// and the command reports a cluster that needs no secret at all. A loud error puts the run on the
// path that creates the secrets of each component.
func TestLoadSchemaRejectsASchemaWithNoShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		schema string
	}{
		{"an object that rejects each field", `{"type": "object", "additionalProperties": false}`},
		{"an object that holds other fields and no more", `{
			"type": "object",
			"additionalProperties": false,
			"properties": { "kind": { "type": "string" } }
		}`},
		{"an object that holds a cluster that takes no field", `{
			"type": "object",
			"additionalProperties": false,
			"properties": { "spec": { "type": "object", "additionalProperties": false } }
		}`},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			_, err := loadSchemaFrom(t, test.schema)

			require.ErrorIs(t, err, secrets.ErrUnreadableSchema)
		})
	}
}

// TestLoadSchemaTakesASchemaThatAcceptsEachField keeps the answer for a schema that takes a field
// with any name. An object that says nothing about the names that it holds takes each of them, thus
// such a schema answers yes to every path. The run then creates the secrets of each component and
// drops none, and the anchor of LoadSchema has no reason to stop it: an extra secret is a file that
// nothing reads.
func TestLoadSchemaTakesASchemaThatAcceptsEachField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		schema string
	}{
		{"a schema of true", `true`},
		{"an object that takes each field", `{"type": "object", "additionalProperties": true}`},
		{"an empty object", `{}`},
		{"an object with no field of its own", `{"type": "object"}`},
		{"an object that holds other fields", `{"type": "object", "properties": {"kind": {"type": "string"}}}`},
		{"an object that holds a cluster and takes each field", `{
			"type": "object",
			"properties": { "spec": { "type": "object" } }
		}`},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			schema, err := loadSchemaFrom(t, test.schema)
			require.NoError(t, err)

			assert.True(t, schema.Has("spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET"))
			assert.True(t, schema.Has("anything.at.all"))
		})
	}
}

// loadSchemaFrom writes one schema as the public schema of the OnPremises kind, and reads it. The
// caller asserts on the error of the package, thus the error comes back as it is.
//
//nolint:wrapcheck // The caller asserts on the error of the package.
func loadSchemaFrom(t *testing.T, schema string) (*secrets.Schema, error) {
	t.Helper()

	repoPath := t.TempDir()
	folder := filepath.Join(repoPath, "schemas", "public")
	require.NoError(t, os.MkdirAll(folder, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(folder, "onpremises-kfd-v1alpha2.json"), []byte(schema), 0o600))

	return secrets.LoadSchema(repoPath, distroconf.Furyctl{
		APIVersion: "kfd.sighup.io/v1alpha2",
		Kind:       "OnPremises",
	})
}

// TestUnreadableFollowsTheFieldsOfTheCluster covers the rule that decides which fields the gate
// reads. A component that Select left out because furyctl could not read its field must name that
// field: the answer of the field is the reason the component is not there. A component that this
// cluster has no field for must name nothing, or a value left over in a configuration file stops a
// run over a component that the cluster cannot use.
func TestUnreadableFollowsTheFieldsOfTheCluster(t *testing.T) {
	t.Parallel()

	// A cluster that holds the Keepalived of the load balancers, and not the one of the control
	// plane. The OnPremises kind is such a cluster.
	const schemaWithKeepalived = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "kind": { "type": "string" },
    "spec": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "distribution": { "type": "object" },
        "kubernetes": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "loadBalancers": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "keepalived": {
                  "type": "object",
                  "additionalProperties": false,
                  "properties": {
                    "enabled": { "type": "boolean" },
                    "passphrase": { "type": "string" }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

	tests := []struct {
		desc  string
		field string
		names bool
	}{
		{
			desc:  "a field of a component that the cluster can hold",
			field: "spec.kubernetes.loadBalancers.keepalived.enabled",
			names: true,
		},
		{
			desc:  "a field of a component that the cluster cannot hold",
			field: "spec.kubernetes.controlPlane.keepalived.enabled",
			names: false,
		},
	}

	schema, err := loadSchemaFrom(t, schemaWithKeepalived)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "furyctl.yaml")
			require.NoError(t, os.WriteFile(path, []byte(nest(test.field)), 0o600))

			cfg, err := secrets.LoadConfig(path)
			require.NoError(t, err)

			components, err := secrets.Select(nil, cfg)
			require.NoError(t, err)

			components, _ = secrets.Keep(components, schema)

			if test.names {
				assert.Equal(t, []string{test.field}, secrets.Unreadable(components, cfg, schema, nil))

				return
			}

			assert.Empty(t, secrets.Unreadable(components, cfg, schema, nil))
		})
	}
}

// nest writes a configuration file that holds one field with a value that furyctl cannot read.
func nest(field string) string {
	lines := []string{"kind: OnPremises"}
	parts := strings.Split(field, ".")

	for i, part := range parts {
		indent := strings.Repeat("  ", i)

		if i == len(parts)-1 {
			lines = append(lines, indent+part+`: "{env://FURYCTL_UNSET_ON_PURPOSE}"`)

			continue
		}

		lines = append(lines, indent+part+":")
	}

	return strings.Join(lines, "\n")
}
