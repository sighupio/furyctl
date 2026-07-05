// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // exercises unexported version-resolution helpers.

import (
	"os"
	"path/filepath"
	"testing"
)

const testManifest = `---
kubernetes:
  1.34.8:
    imageRegistry: registry.sighup.io/fury/on-premises
    sandboxTag: "3.10.1"
    corednsImagePrefix: /coredns
    haproxyImage: registry.sighup.io/fury/on-premises/haproxy
    haproxyTag: "3.0.6"
    sysext:
      - name: containerd
        version: 2.3.1
        arch:
          x86-64:
            url: https://example/containerd-2.3.1-x86-64.raw
      - name: kubernetes
        version: v1.34.8
        arch:
          x86-64:
            url: https://example/kubernetes-v1.34.8-x86-64.raw
    flatcar:
      version: 4593.2.1
      channel: stable
      arch:
        x86-64:
          kernel:
            filename: k
            url: https://example/k
`

// writeManifest lays out a phase dir with a sibling vendor/installers/immutable/immutable.yaml.
func writeManifest(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	phaseDir := filepath.Join(base, "kubernetes")
	manifestDir := filepath.Join(base, "vendor", "installers", "immutable")

	if err := os.MkdirAll(phaseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(manifestDir, "immutable.yaml"), []byte(testManifest), 0o600); err != nil {
		t.Fatal(err)
	}

	return phaseDir
}

func TestSelectImmutableAssets(t *testing.T) {
	t.Parallel()

	phaseDir := writeManifest(t)

	// The version is used as-is (it comes from kfd.immutable.version, which carries no leading "v").
	got, err := selectImmutableAssets(phaseDir, "1.34.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.SandboxTag != "3.10.1" {
		t.Errorf("SandboxTag = %q", got.SandboxTag)
	}

	if got.ImageRegistry != "registry.sighup.io/fury/on-premises" {
		t.Errorf("ImageRegistry = %q", got.ImageRegistry)
	}

	if got.CorednsImagePrefix != "/coredns" {
		t.Errorf("CorednsImagePrefix = %q", got.CorednsImagePrefix)
	}

	// A version absent from the manifest is an error.
	if _, err := selectImmutableAssets(phaseDir, "9.99.99"); err == nil {
		t.Error("expected error for missing version, got nil")
	}
}

// TestBuildVersionVars checks the infra roles' required vars (sandbox tag, haproxy image/tag) are emitted.
func TestBuildVersionVars(t *testing.T) {
	t.Parallel()

	phaseDir := writeManifest(t)

	a, err := selectImmutableAssets(phaseDir, "1.34.8")
	if err != nil {
		t.Fatal(err)
	}

	vars := buildVersionVars("1.34.8", "/usr/bin/kubectl", a)

	want := map[string]string{
		"containerd_sandbox_tag":  "3.10.1",
		"haproxy_container_image": "registry.sighup.io/fury/on-premises/haproxy",
		"haproxy_container_tag":   "3.0.6",
	}

	for key, exp := range want {
		got, ok := vars[key]
		if !ok {
			t.Errorf("buildVersionVars missing %q", key)

			continue
		}

		if got != exp {
			t.Errorf("buildVersionVars[%q] = %v, want %q", key, got, exp)
		}
	}
}
