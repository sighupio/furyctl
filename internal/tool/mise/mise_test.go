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

	for _, n := range []string{"kubectl", "opentofu", "furyagent", "terraform"} {
		if !IsManaged(n) {
			t.Errorf("expected %s to be managed", n)
		}
	}

	for _, n := range []string{"awscli", "ansible", "git"} {
		if IsManaged(n) {
			t.Errorf("expected %s NOT to be managed", n)
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
	})
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
		`"ubi:sighupio/furyagent" = "0.4.0"`,
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
