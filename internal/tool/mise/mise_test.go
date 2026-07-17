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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err, "AssetName(%s)", osArch)

		assert.Equal(t, want, got, "AssetName(%s)", osArch)
	}

	_, err := AssetName("windows", "amd64")
	assert.Error(t, err, "expected error for unsupported platform")
}

func Test_DownloadURL(t *testing.T) {
	t.Parallel()

	url, err := DownloadURL("linux", "amd64")
	require.NoError(t, err, "DownloadURL")

	assert.Contains(t, url, "mise-"+Version+"-linux-x64-musl", "url missing asset")
	assert.Contains(t, url, "?checksum=sha256:"+binChecksums["linux/amd64"], "url missing checksum")

	_, err = DownloadURL("plan9", "arm64")
	assert.Error(t, err, "expected error for unsupported platform")
}

func Test_IsManaged(t *testing.T) {
	t.Parallel()

	for _, n := range []string{"kubectl", "opentofu", "furyagent", "terraform", "ansible"} {
		assert.True(t, IsManaged(n), "expected %s to be managed", n)
	}

	for _, n := range []string{"awscli", "git"} {
		assert.False(t, IsManaged(n), "expected %s NOT to be managed", n)
	}
}

func Test_WriteConfig_Ansible(t *testing.T) {
	t.Parallel()

	// Distribution-pinned uv/python win over the built-in defaults.
	pathPinned := filepath.Join(t.TempDir(), "mise.toml")

	err := WriteConfig(pathPinned, map[string]string{"ansible": "2.21.0"}, "0.8", "3.13")
	require.NoError(t, err, "WriteConfig")

	pinned, err := os.ReadFile(pathPinned)
	require.NoError(t, err, "read")

	for _, want := range []string{
		`"uv" = "0.8"`,
		`"python" = "3.13"`,
		`"pipx:ansible-core" = "2.21.0"`,
	} {
		assert.Contains(t, string(pinned), want, "pinned config missing %q\n%s", want, string(pinned))
	}

	// Empty uv/python fall back to the built-in defaults.
	pathDefault := filepath.Join(t.TempDir(), "mise.toml")

	err = WriteConfig(pathDefault, map[string]string{"ansible": "2.21.0"}, "", "")
	require.NoError(t, err, "WriteConfig")

	def, err := os.ReadFile(pathDefault)
	require.NoError(t, err, "read")

	for _, want := range []string{
		`"uv" = "` + AnsibleUvVersion + `"`,
		`"python" = "` + AnsiblePythonVersion + `"`,
	} {
		assert.Contains(t, string(def), want, "default config missing %q\n%s", want, string(def))
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
	require.NoError(t, err, "WriteConfig")

	b, err := os.ReadFile(path)
	require.NoError(t, err, "read")

	content := string(b)

	for _, want := range []string{
		"[tools]",
		`"kubectl" = "1.34.4"`,
		`"opentofu" = "1.10.0"`,
		`"github:sighupio/furyagent" = "0.4.0"`,
	} {
		assert.Contains(t, content, want, "config missing %q\n%s", want, content)
	}

	assert.NotContains(t, content, "awscli", "unmanaged awscli leaked into config")
}

func Test_parseEnvJSON(t *testing.T) {
	t.Parallel()

	got, err := parseEnvJSON(`{"PATH":"/a:/b","FOO":"bar"}`)
	require.NoError(t, err, "parseEnvJSON")

	assert.Equal(t, []string{"FOO=bar", "PATH=/a:/b"}, got)

	_, err = parseEnvJSON("not json")
	assert.Error(t, err, "expected error for invalid json")
}
