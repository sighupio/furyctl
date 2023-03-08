// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legacy

import (
	"fmt"
	"path"
	"strings"
)

type PackageURL struct {
	Prefix        string
	Blocks        []string
	Kind          string
	Version       string
	Registry      bool
	CloudProvider ProviderOptSpec
	KindSpec      ProviderKind
}

func newPackageURL(
	prefix string,
	blocks []string,
	kind,
	version string,
	registry bool,
	cloud ProviderOptSpec,
	kindSpec ProviderKind,
) *PackageURL {
	return &PackageURL{
		Prefix:        prefix,
		Registry:      registry,
		Blocks:        blocks,
		Kind:          kind,
		Version:       version,
		CloudProvider: cloud,
		KindSpec:      kindSpec,
	}
}

func (n *PackageURL) getConsumableURL() string {
	if !n.Registry {
		return n.getURLFromCompanyRepos()
	}

	return fmt.Sprintf("%s/%s%s?ref=%s", n.KindSpec.pickCloudProviderURL(n.CloudProvider), n.Blocks[0], ".git", n.Version)
}

func (n *PackageURL) getURLFromCompanyRepos() string {
	if len(n.Blocks) == 0 {
		return ""
	}

	dG := ""

	if strings.HasPrefix(n.Prefix, "git::https") {
		dG = ".git"
	}

	if len(n.Blocks) == 1 {
		return fmt.Sprintf("%s-%s%s//%s?ref=%s", n.Prefix, n.Blocks[0], dG, n.Kind, n.Version)
	}

	remainingBlocks := ""

	for i := 1; i < len(n.Blocks); i++ {
		remainingBlocks = path.Join(remainingBlocks, n.Blocks[i])
	}

	return fmt.Sprintf("%s-%s%s//%s/%s?ref=%s", n.Prefix, n.Blocks[0], dG, n.Kind, remainingBlocks, n.Version)
}
