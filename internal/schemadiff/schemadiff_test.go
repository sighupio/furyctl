// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package schemadiff_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/schemadiff"
)

func load(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "reading %s", name)

	return raw
}

// TestCompareFindsTheRealBreakingChanges pins the two changes that motivated this package,
// both taken from real SD releases:
//   - kubeProxy.enabled was replaced by kubeProxy.type in v1.35.0, inside an inline object
//   - logging.customOutputs gained a required ingressHaproxy key in v1.34.0, which the
//     release notes do not mention at all
func TestCompareFindsTheRealBreakingChanges(t *testing.T) {
	t.Parallel()

	changes, err := schemadiff.Compare(load(t, "current.json"), load(t, "target.json"))
	require.NoError(t, err, "Compare")

	assert.Contains(t, changes, schemadiff.Change{
		Definition: "Spec.Kubernetes.Advanced.kubeProxy",
		Property:   "enabled",
		Kind:       schemadiff.PropertyRemoved,
	}, "a property removed from an inline nested object must be found")

	assert.Contains(t, changes, schemadiff.Change{
		Definition: "Spec.Kubernetes.Advanced.kubeProxy",
		Property:   "type",
		Kind:       schemadiff.PropertyAdded,
	}, "its replacement is reported too, which is what makes the rename recognisable")

	assert.Contains(t, changes, schemadiff.Change{
		Definition: "Spec.Distribution.Modules.Logging.CustomOutputs",
		Property:   "ingressHaproxy",
		Kind:       schemadiff.RequiredAdded,
	}, "a newly required property must be found")
}

func TestCompareReportsAdditionsAndIgnoresNewDefinitions(t *testing.T) {
	t.Parallel()

	changes, err := schemadiff.Compare(load(t, "current.json"), load(t, "target.json"))
	require.NoError(t, err, "Compare")

	assert.Contains(t, changes, schemadiff.Change{
		Definition: "Spec.Kubernetes.Advanced.Encryption",
		Property:   "tlsCipherSuitesKubelet",
		Kind:       schemadiff.PropertyAdded,
	}, "a new optional property is reported as an addition")

	// Spec.Distribution.Modules.Ingress.HAProxy exists only in the target. It describes
	// configuration nobody could have written before, so it must not be reported.
	for _, change := range changes {
		assert.NotEqual(t, "Spec.Distribution.Modules.Ingress.HAProxy", change.Definition,
			"a definition that only exists in the target is not a change to an existing config")
	}
}

func TestCompareNoChanges(t *testing.T) {
	t.Parallel()

	current := load(t, "current.json")

	changes, err := schemadiff.Compare(current, current)
	require.NoError(t, err, "Compare")
	assert.Empty(t, changes, "a schema compared with itself has no changes")
}

func TestCompareIsDeterministic(t *testing.T) {
	t.Parallel()

	current, target := load(t, "current.json"), load(t, "target.json")

	first, err := schemadiff.Compare(current, target)
	require.NoError(t, err, "Compare")

	// Map iteration order must not leak into the output, or the report would shuffle
	// between runs.
	for range 5 {
		again, err := schemadiff.Compare(current, target)
		require.NoError(t, err, "Compare")
		assert.Equal(t, first, again, "repeated comparisons must agree")
	}
}

func TestCompareInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := schemadiff.Compare([]byte("{not json"), load(t, "target.json"))
	require.Error(t, err, "Compare")
	assert.Contains(t, err.Error(), "current schema", "the error must say which side is broken")

	_, err = schemadiff.Compare(load(t, "current.json"), []byte("{not json"))
	require.Error(t, err, "Compare")
	assert.Contains(t, err.Error(), "target schema", "the error must say which side is broken")
}
