// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

// Per-kind dependency requirements: which tools / modules / installers a given cluster Kind
// actually needs. Used to avoid downloading and validating dependencies that are irrelevant to
// the provider (e.g. terraform/awscli on OnPremises, the eks/immutable installers on OnPremises).
// Tool/module/installer names are the lowercased struct field names from the kfd config.

// ToolNeededForKind reports whether the given tool is needed for the given cluster kind.
// Provider-specific tools are gated to their kind; everything else is common to every kind.
func ToolNeededForKind(tool, kind string) bool {
	switch tool {
	case "terraform", "opentofu", "furyagent", "awscli":
		return kind == EKSClusterKind

	case "ansible":
		return kind == OnPremisesKind || kind == ImmutableKind

	default:
		return true
	}
}

// ModuleNeededForKind reports whether the given distribution module is needed for the kind.
func ModuleNeededForKind(module, kind string) bool {
	if module == "aws" {
		return kind == EKSClusterKind
	}

	return true
}

// InstallerNeededForKind reports whether the named installer (a kfd.Kubernetes field: eks,
// onpremises, immutable) is needed for the given cluster kind. KFDDistribution needs none.
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
