// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import "github.com/sighupio/furyctl/internal/apis/config"

// Per-kind dependency requirements: which tool sections / modules / installers a given cluster Kind
// actually needs. Used to avoid downloading and validating dependencies that are irrelevant to the
// provider (e.g. the eks tools/installer on OnPremises). Names are the lowercased struct field names
// from the kfd config.

// EffectiveOpenTofuVersion resolves the OpenTofu version with provider-overrides-common semantics:
// the value pinned under the provider section (tools.eks, distributions >= 1.34.2) wins, otherwise
// it falls back to tools.common (distributions < 1.34.2). Empty if OpenTofu is not pinned at all
// (very old distributions that only ship terraform).
func EffectiveOpenTofuVersion(tools config.KFDTools) string {
	if tools.Eks.OpenTofu.Version != "" {
		return tools.Eks.OpenTofu.Version
	}

	return tools.Common.OpenTofu.Version
}

// EffectiveFuryagentVersion resolves the Furyagent version with provider-overrides-common semantics
// (tools.eks wins, falls back to tools.common).
func EffectiveFuryagentVersion(tools config.KFDTools) string {
	if tools.Eks.Furyagent.Version != "" {
		return tools.Eks.Furyagent.Version
	}

	return tools.Common.Furyagent.Version
}

// ToolSectionNeededForKind reports whether a tools section (a kfd.Tools field: common, eks) is needed
// for the given cluster kind. The common section is always needed; provider sections only for their kind.
func ToolSectionNeededForKind(section, kind string) bool {
	// The common section is needed by every kind; provider sections only by their own kind.
	if section == "eks" {
		return kind == EKSClusterKind
	}

	return true
}

// ModuleNeededForKind reports whether the given distribution module is needed for the kind.
func ModuleNeededForKind(module, kind string) bool {
	if module == "aws" {
		return kind == EKSClusterKind
	}

	return true
}

// InstallerNeededForKind reports whether the named installer (a kfd.Kubernetes field: eks, onpremises,
// immutable) is needed for the given cluster kind. KFDDistribution needs none.
func InstallerNeededForKind(installer, kind string) bool {
	switch installer {
	case "eks":
		return kind == EKSClusterKind

	case "onpremises":
		return kind == OnPremisesKind

	case "immutable":
		return kind == ImmutableKind

	default:
		return false
	}
}
