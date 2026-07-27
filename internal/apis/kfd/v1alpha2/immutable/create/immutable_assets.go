// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// selectImmutableAssets loads the vendored immutable.yaml and selects the block for the given
// kubernetes version. It is the single selector used by every furyctl phase (infrastructure and
// kubernetes), so baked artifacts and rendered role variables never disagree on version. A free
// function (no Infrastructure receiver) so both phases call it directly. The version is passed as-is
// (it comes from the kfd manifest, which we control), so a mismatch surfaces as
// ErrKubernetesVersionNotFound; an empty manifest surfaces as ErrNoKubernetesVersions.
func selectImmutableAssets(phasePath, kubeVersion string) (assets, error) {
	immutableSpecPath := filepath.Join(phasePath, "..", "vendor", "installers", "immutable", "immutable.yaml")

	data, err := os.ReadFile(immutableSpecPath)
	if err != nil {
		return assets{}, fmt.Errorf("error reading immutable manifest at %s: %w", immutableSpecPath, err)
	}

	var manifest immutableManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return assets{}, fmt.Errorf("error parsing immutable manifest: %w", err)
	}

	if len(manifest.Kubernetes) == 0 {
		return assets{}, ErrNoKubernetesVersions
	}

	immutableAssets, ok := manifest.Kubernetes[kubeVersion]
	if !ok {
		return assets{}, fmt.Errorf("%w: %s", ErrKubernetesVersionNotFound, kubeVersion)
	}

	// Empty tag/registry would render "<registry>/pause:" and brick every pod; fail loudly (also catches skew).
	if immutableAssets.SandboxTag == "" || immutableAssets.ImageRegistry == "" {
		return assets{}, fmt.Errorf("%w: kubernetes %s", ErrSandboxTagOrRegistryEmpty, kubeVersion)
	}

	return immutableAssets, nil
}

// VersionVarsForPhase resolves the immutable.yaml pins for kubeVersion (from phasePath/../vendor)
// into the "versions" template data shared by the create phases and the certificates renewer.
func VersionVarsForPhase(phasePath, kubeVersion, kubectlBin string) (map[any]any, error) {
	immutableAssets, err := selectImmutableAssets(phasePath, kubeVersion)
	if err != nil {
		return nil, fmt.Errorf("error selecting immutable assets: %w", err)
	}

	return versionVarsFromAssets(kubeVersion, kubectlBin, immutableAssets), nil
}
