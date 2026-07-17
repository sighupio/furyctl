// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/distribution"
)

func Test_ToolSectionNeededForKind(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		section string
		kind    string
		want    bool
	}{
		{"common", distribution.EKSClusterKind, true},
		{"common", distribution.OnPremisesKind, true},
		{"common", distribution.KFDDistributionKind, true},
		{"eks", distribution.EKSClusterKind, true},
		{"eks", distribution.OnPremisesKind, false},
		{"eks", distribution.ImmutableKind, false},
		{"eks", distribution.KFDDistributionKind, false},
	}

	for _, tC := range testCases {
		got := distribution.ToolSectionNeededForKind(tC.section, tC.kind)
		assert.Equal(t, tC.want, got, "ToolSectionNeededForKind(%q, %q)", tC.section, tC.kind)
	}
}

func Test_ModuleNeededForKind(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		module string
		kind   string
		want   bool
	}{
		{"aws", distribution.EKSClusterKind, true},
		{"aws", distribution.OnPremisesKind, false},
		{"aws", distribution.KFDDistributionKind, false},
		{"monitoring", distribution.EKSClusterKind, true},
		{"monitoring", distribution.OnPremisesKind, true},
	}

	for _, tC := range testCases {
		got := distribution.ModuleNeededForKind(tC.module, tC.kind)
		assert.Equal(t, tC.want, got, "ModuleNeededForKind(%q, %q)", tC.module, tC.kind)
	}
}

func Test_InstallerNeededForKind(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		installer string
		kind      string
		want      bool
	}{
		{"eks", distribution.EKSClusterKind, true},
		{"eks", distribution.OnPremisesKind, false},
		{"onpremises", distribution.OnPremisesKind, true},
		{"onpremises", distribution.EKSClusterKind, false},
		{"immutable", distribution.ImmutableKind, true},
		{"immutable", distribution.OnPremisesKind, false},
		{"eks", distribution.KFDDistributionKind, false},
		{"onpremises", distribution.KFDDistributionKind, false},
	}

	for _, tC := range testCases {
		got := distribution.InstallerNeededForKind(tC.installer, tC.kind)
		assert.Equal(t, tC.want, got, "InstallerNeededForKind(%q, %q)", tC.installer, tC.kind)
	}
}
