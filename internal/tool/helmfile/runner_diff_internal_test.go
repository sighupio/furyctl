// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package helmfile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckReadOnlyDiffArgs_TheDiffCommandPasses(t *testing.T) {
	t.Parallel()

	require.NoError(t, checkReadOnlyDiffArgs(diffArgs()))
}

func TestCheckReadOnlyDiffArgs_RefusesArgumentsThatAreNotAReadOnlyDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		reason string
	}{
		{"no arguments", nil, "the subcommand is not diff"},
		{"apply", []string{"apply", "--suppress-secrets"}, "the subcommand is not diff"},
		{"sync", []string{"sync"}, "the subcommand is not diff"},
		{"destroy", []string{"destroy"}, "the subcommand is not diff"},
		{"diff then apply", []string{"diff", "--suppress-secrets", "apply"}, "it names apply"},
		{"diff then sync", []string{"diff", "sync", "--suppress-secrets"}, "it names sync"},
		{"diff then destroy", []string{"diff", "--suppress-secrets", "destroy"}, "it names destroy"},
		{"secrets shown", []string{"diff", "--output", "simple", "--no-color"}, "it does not pass --suppress-secrets"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkReadOnlyDiffArgs(tt.args)

			require.ErrorIs(t, err, ErrDiffNotReadOnly)
			require.EqualError(t, err, "the helmfile diff command is not read-only: "+tt.reason)
		})
	}
}
