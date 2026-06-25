// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // parseEnvJSON is unexported.
package mise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_AssetName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"linux/amd64":  "mise-" + Version + "-linux-x64-musl",
		"linux/arm64":  "mise-" + Version + "-linux-arm64-musl",
		"darwin/amd64": "mise-" + Version + "-macos-x64",
		"darwin/arm64": "mise-" + Version + "-macos-arm64",
	}

	for osArch, want := range cases {
		parts := strings.SplitN(osArch, "/", 2)

		got, err := AssetName(parts[0], parts[1])
		if err != nil {
			t.Fatalf("AssetName(%s) error: %v", osArch, err)
		}

		if got != want {
			t.Errorf("AssetName(%s) = %q, want %q", osArch, got, want)
		}
	}

	if _, err := AssetName("windows", "amd64"); err == nil {
		t.Error("expected error for unsupported platform")
	}
}

func Test_DownloadURL(t *testing.T) {
	t.Parallel()

	url, err := DownloadURL("linux", "amd64")
	if err != nil {
		t.Fatalf("DownloadURL error: %v", err)
	}

	if !strings.Contains(url, "mise-"+Version+"-linux-x64-musl") {
		t.Errorf("url missing asset: %s", url)
	}

	if !strings.Contains(url, "?checksum=sha256:"+binChecksums["linux/amd64"]) {
		t.Errorf("url missing checksum: %s", url)
	}

	if _, err := DownloadURL("plan9", "arm64"); err == nil {
		t.Error("expected error for unsupported platform")
	}
}

func Test_IsManaged(t *testing.T) {
	t.Parallel()

	for _, n := range []string{"kubectl", "opentofu", "furyagent", "terraform", "ansible"} {
		if !IsManaged(n) {
			t.Errorf("expected %s to be managed", n)
		}
	}

	for _, n := range []string{"awscli", "git"} {
		if IsManaged(n) {
			t.Errorf("expected %s NOT to be managed", n)
		}
	}
}

func Test_WriteConfig_Ansible(t *testing.T) {
	t.Parallel()

	// Distribution-pinned uv/python win over the built-in defaults.
	pathPinned := filepath.Join(t.TempDir(), "mise.toml")

	if err := WriteConfig(pathPinned, map[string]string{"ansible": "2.21.0"}, "0.8", "3.13"); err != nil {
		t.Fatalf("WriteConfig error: %v", err)
	}

	pinned, err := os.ReadFile(pathPinned)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	for _, want := range []string{
		`"uv" = "0.8"`,
		`"python" = "3.13"`,
		`"pipx:ansible-core" = "2.21.0"`,
	} {
		if !strings.Contains(string(pinned), want) {
			t.Errorf("pinned config missing %q\n%s", want, string(pinned))
		}
	}

	// Empty uv/python fall back to the built-in defaults.
	pathDefault := filepath.Join(t.TempDir(), "mise.toml")

	if err := WriteConfig(pathDefault, map[string]string{"ansible": "2.21.0"}, "", ""); err != nil {
		t.Fatalf("WriteConfig error: %v", err)
	}

	def, err := os.ReadFile(pathDefault)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	for _, want := range []string{
		`"uv" = "` + AnsibleUvVersion + `"`,
		`"python" = "` + AnsiblePythonVersion + `"`,
	} {
		if !strings.Contains(string(def), want) {
			t.Errorf("default config missing %q\n%s", want, string(def))
		}
	}
}

func Test_WriteConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "mise.toml")

	err := WriteConfig(path, map[string]string{
		"kubectl":   "1.34.4",
		"opentofu":  "1.10.0",
		"furyagent": "0.4.0",
		"awscli":    "2.8.12", // not managed -> must be skipped
	}, "", "")
	if err != nil {
		t.Fatalf("WriteConfig error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	content := string(b)

	for _, want := range []string{
		"[tools]",
		`"kubectl" = "1.34.4"`,
		`"opentofu" = "1.10.0"`,
		`"github:sighupio/furyagent" = "0.4.0"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("config missing %q\n%s", want, content)
		}
	}

	if strings.Contains(content, "awscli") {
		t.Errorf("unmanaged awscli leaked into config:\n%s", content)
	}
}

func Test_parseEnvJSON(t *testing.T) {
	t.Parallel()

	got, err := parseEnvJSON(`{"PATH":"/a:/b","FOO":"bar"}`)
	if err != nil {
		t.Fatalf("parseEnvJSON error: %v", err)
	}

	want := []string{"FOO=bar", "PATH=/a:/b"} // sorted
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("parseEnvJSON = %v, want %v", got, want)
	}

	if _, err := parseEnvJSON("not json"); err == nil {
		t.Error("expected error for invalid json")
	}
}
