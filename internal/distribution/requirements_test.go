// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/distribution"
)

func TestToolNeededForKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool string
		kind string
		want bool
	}{
		{"kubectl", distribution.OnPremisesKind, true},
		{"kubectl", distribution.EKSClusterKind, true},
		{"kustomize", distribution.KFDDistributionKind, true},
		{"terraform", distribution.EKSClusterKind, true},
		{"terraform", distribution.OnPremisesKind, false},
		{"opentofu", distribution.OnPremisesKind, false},
		{"furyagent", distribution.EKSClusterKind, true},
		{"furyagent", distribution.OnPremisesKind, false},
		{"awscli", distribution.EKSClusterKind, true},
		{"awscli", distribution.OnPremisesKind, false},
		{"ansible", distribution.OnPremisesKind, true},
		{"ansible", distribution.ImmutableKind, true},
		{"ansible", distribution.EKSClusterKind, false},
		{"ansible", distribution.KFDDistributionKind, false},
		{"butane", distribution.ImmutableKind, true}, // not a kfd tool name -> treated as common
	}
	for _, c := range cases {
		if got := distribution.ToolNeededForKind(c.tool, c.kind); got != c.want {
			t.Errorf("ToolNeededForKind(%q,%q) = %v, want %v", c.tool, c.kind, got, c.want)
		}
	}
}

func TestModuleNeededForKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		module string
		kind   string
		want   bool
	}{
		{"aws", distribution.EKSClusterKind, true},
		{"aws", distribution.OnPremisesKind, false},
		{"ingress", distribution.OnPremisesKind, true},
		{"monitoring", distribution.KFDDistributionKind, true},
	}
	for _, c := range cases {
		if got := distribution.ModuleNeededForKind(c.module, c.kind); got != c.want {
			t.Errorf("ModuleNeededForKind(%q,%q) = %v, want %v", c.module, c.kind, got, c.want)
		}
	}
}

func TestInstallerNeededForKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
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
		{"immutable", distribution.EKSClusterKind, false},
	}
	for _, c := range cases {
		if got := distribution.InstallerNeededForKind(c.installer, c.kind); got != c.want {
			t.Errorf("InstallerNeededForKind(%q,%q) = %v, want %v", c.installer, c.kind, got, c.want)
		}
	}
}
