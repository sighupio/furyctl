// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sighupio/furyctl/internal/git"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const immutableKind = "Immutable"

var ErrImmutableVersionNotInManifest = errors.New(
	"kfd.immutable.version is not declared in the immutable installer manifest (immutable.yaml)",
)

// immutableManifestVersions is the minimal view of immutable.yaml needed to check the invariant:
// kfd.immutable.version (normalized) must be a key of the kubernetes map.
type immutableManifestVersions struct {
	Kubernetes map[string]any `yaml:"kubernetes"`
}

// validateImmutableVersion enforces fail-fast at `furyctl validate`: the selected version
// (normalize(kfd.immutable.version)) must be a key of the vendored immutable.yaml. The manifest
// lives in installer-immutable@<kfd.immutable.installer>, which validate does not otherwise fetch,
// so it is cloned here into a temp dir.
func validateImmutableVersion(res dist.DownloadResult, gitProtocol git.Protocol) error {
	version := strings.TrimPrefix(res.DistroManifest.Kubernetes.Immutable.Version, "v")
	installerRef := res.DistroManifest.Kubernetes.Immutable.Installer

	prefix, err := git.RepoPrefixByProtocol(gitProtocol)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	dst, err := os.MkdirTemp("", "furyctl-immutable-")
	if err != nil {
		return fmt.Errorf("error creating temp dir for immutable manifest: %w", err)
	}

	defer func() { _ = os.RemoveAll(dst) }()

	src := fmt.Sprintf("git::%s/installer-immutable?ref=%s&depth=1", prefix, installerRef)

	client := netx.NewGoGetterClient()
	if err := client.Download(src, dst); err != nil {
		return fmt.Errorf("error downloading immutable installer manifest for validation: %w", err)
	}

	manifest, err := yamlx.FromFileV3[immutableManifestVersions](filepath.Join(dst, "immutable.yaml"))
	if err != nil {
		return fmt.Errorf("error reading immutable.yaml: %w", err)
	}

	if _, ok := manifest.Kubernetes[version]; ok {
		return nil
	}

	available := make([]string, 0, len(manifest.Kubernetes))
	for k := range manifest.Kubernetes {
		available = append(available, k)
	}

	sort.Strings(available)

	return fmt.Errorf("%w: %q (available versions: %s)",
		ErrImmutableVersionNotInManifest, version, strings.Join(available, ", "))
}
