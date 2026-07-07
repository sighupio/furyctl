// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"testing"

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
		if got := distribution.ToolSectionNeededForKind(tC.section, tC.kind); got != tC.want {
			t.Errorf("ToolSectionNeededForKind(%q, %q) = %v, want %v", tC.section, tC.kind, got, tC.want)
		}
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
		if got := distribution.ModuleNeededForKind(tC.module, tC.kind); got != tC.want {
			t.Errorf("ModuleNeededForKind(%q, %q) = %v, want %v", tC.module, tC.kind, got, tC.want)
		}
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
		if got := distribution.InstallerNeededForKind(tC.installer, tC.kind); got != tC.want {
			t.Errorf("InstallerNeededForKind(%q, %q) = %v, want %v", tC.installer, tC.kind, got, tC.want)
		}
	}
}
