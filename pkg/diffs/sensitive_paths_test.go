// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"path/filepath"
	"strings"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// secretSentinel is the fake value every secret of these tests holds.
const secretSentinel = "sentinel-secret-8d21"

// secretFields are fields that hold a credential although no word of their key says so,
// one for each secret path, with list items at position 0.
func secretFields() []string {
	return []string{
		"spec.plugins.helm.releases.0.set.0.value",
		"spec.distribution.modules.auth.dex.connectors.0.config.bindPW",
		"spec.distribution.modules.monitoring.alertmanager.slackWebhookUrl",
		"spec.distribution.modules.monitoring.alertmanager.deadManSwitchWebhookUrl",
		"spec.distribution.modules.monitoring.prometheus.remoteWrite.0.headers.X-Api-Auth",
		"spec.distribution.modules.monitoring.prometheusAgent.remoteWrite.0.authorization.credentials",
		"spec.distribution.modules.logging.customOutputs.audit",
		"spec.distribution.customPatches.patches.0.patch",
		"spec.distribution.customPatches.patchesStrategicMerge.0",
		"spec.kubernetes.advanced.encryption.configuration",
		"spec.kubernetes.advanced.cloud.config",
		"spec.kubernetes.loadBalancers.additionalConfig",
		"spec.infrastructure.loadBalancers.haproxy.configuration",
		"spec.infrastructure.ipxeServer.preInstallCommands.0",
		"spec.infrastructure.ipxeServer.postInstallCommands.0",
		"spec.infrastructure.nodes.0.storage.files.0.contents.inline",
		"spec.infrastructure.nodes.0.storage.files.0.contents.source",
		"spec.infrastructure.nodes.0.storage.files.0.contents.http_headers.0.value",
		"spec.infrastructure.nodes.0.storage.files.0.append.0.inline",
		"spec.infrastructure.nodes.0.storage.files.0.append.0.source",
		"spec.infrastructure.nodes.0.storage.files.0.append.0.http_headers.0.value",
		"spec.infrastructure.nodes.0.systemd.units.0.contents",
		"spec.infrastructure.nodes.0.systemd.units.0.dropins.0.contents",
	}
}

// nested gives the value that holds the leaf at the segments below it: an object for
// a key and a list with one item for a position.
func nested(segments []string, leaf any) any {
	if len(segments) == 0 {
		return leaf
	}

	if diffs.IsListPosition(segments[0]) {
		return []any{nested(segments[1:], leaf)}
	}

	return map[string]any{segments[0]: nested(segments[1:], leaf)}
}

func TestIsSensitivePath_SecretPaths(t *testing.T) {
	t.Parallel()

	for _, field := range secretFields() {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			path := strings.Split(field, ".")

			assert.True(t, diffs.IsSensitivePath(path), "the field")
			assert.True(t, diffs.IsSensitivePath(append(path, "nested")), "a field below it")
		})
	}
}

func TestIsSensitivePath_SecretPathsAtAnyPosition(t *testing.T) {
	t.Parallel()

	assert.True(t, diffs.IsSensitivePath(strings.Split("spec.plugins.helm.releases.3.set.12.value", ".")))
	assert.True(
		t,
		diffs.IsSensitivePath(strings.Split("spec.infrastructure.nodes.7.storage.files.2.contents.inline", ".")),
	)
	assert.False(t, diffs.IsSensitivePath(strings.Split("spec.plugins.helm.releases.3.set.12.name", ".")))
}

func TestIsSensitivePath_PlainFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		want bool
	}{
		{"the key of a toleration names a label", "spec.distribution.common.tolerations.0.key", false},
		{"the key of a module toleration", "spec.distribution.modules.ingress.nginx.overrides.tolerations.2.key", false},
		{"the key of a taint names a label", "spec.kubernetes.nodeGroups.0.taints.1.key", false},
		{"the path of the ssh key", "spec.kubernetes.ssh.keyPath", false},
		{"the path of the private ssh key", "spec.infrastructure.ssh.privateKeyPath", false},
		{"the path of the public ssh key", "spec.infrastructure.ssh.publicKeyPath", false},
		{"the instance metadata switch", "spec.kubernetes.nodePoolsCommon.metadataHttpTokens", false},
		{"the prefix of the terraform state", "spec.toolsConfiguration.terraform.state.s3.keyPrefix", false},
		{"the prefix of the opentofu state", "spec.toolsConfiguration.opentofu.state.s3.keyPrefix", false},
		{"a key that is not the key of a toleration item", "spec.distribution.common.tolerations.key", true},
		{"a key field next to tolerations", "spec.distribution.common.key", true},
		{"a key path somewhere else", "spec.plugins.helm.releases.0.keyPath", true},
		{"a plain field below a secret", "spec.distribution.modules.auth.clientSecret.tolerations.0.key", true},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tC.want, diffs.IsSensitivePath(strings.Split(tC.path, ".")))
		})
	}
}

func TestIsSensitivePath_CredentialWords(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"spec.distribution.modules.monitoring.prometheus.remoteWrite.0.authorization.type",
		"spec.distribution.modules.monitoring.prometheus.remoteWrite.0.sigv4.credentialsFile",
	} {
		assert.True(t, diffs.IsSensitivePath(strings.Split(path, ".")), path)
	}
}

// Each secret field is hidden in the apply debug log whatever the change is: the field
// changed, any object above it added or removed, the list item above it added or removed.
func TestDiffToString_SecretPathsInEveryShape(t *testing.T) {
	t.Parallel()

	checker := diffs.NewBaseChecker(nil, nil)

	for _, field := range secretFields() {
		path := strings.Split(field, ".")

		t.Run(field, func(t *testing.T) {
			t.Parallel()

			got := checker.DiffToString(r3diff.Changelog{
				{Type: r3diff.UPDATE, Path: path, From: secretSentinel + "-old", To: secretSentinel + "-new"},
			})

			assert.Equal(t, "."+field+": (hidden) -> (hidden)\n", got)

			// The phase key is the shortest path a whole object is added at.
			for depth := 2; depth < len(path); depth++ {
				value := nested(path[depth:], secretSentinel)

				added := checker.DiffToString(r3diff.Changelog{
					{Type: r3diff.CREATE, Path: path[:depth], From: nil, To: value},
				})
				removed := checker.DiffToString(r3diff.Changelog{
					{Type: r3diff.DELETE, Path: path[:depth], From: value, To: nil},
				})

				assert.NotContains(t, added, secretSentinel, "added at %v", path[:depth])
				assert.NotContains(t, removed, secretSentinel, "removed at %v", path[:depth])
				assert.Contains(t, added, "(hidden)", "added at %v", path[:depth])
			}
		})
	}
}

func TestMaskSensitive_HelmSetValue(t *testing.T) {
	t.Parallel()

	release := map[string]any{
		"name":  "app",
		"chart": "example/app",
		"set": []any{
			map[string]any{"name": "auth.adminPassword", "value": secretSentinel},
			map[string]any{"name": "replicas", "value": 3},
		},
	}

	got := diffs.MaskSensitive([]string{"spec", "plugins", "helm", "releases", "0"}, release)

	assert.Equal(
		t,
		map[string]any{
			"name":  "app",
			"chart": "example/app",
			"set": []any{
				map[string]any{"name": "auth.adminPassword", "value": "(hidden)"},
				map[string]any{"name": "replicas", "value": "(hidden)"},
			},
		},
		got,
	)
	assert.Equal(t, secretSentinel, release["set"].([]any)[0].(map[string]any)["value"])
}

func TestMaskSensitive_SensitivePath(t *testing.T) {
	t.Parallel()

	got := diffs.MaskSensitive(
		[]string{"spec", "kubernetes", "advanced", "encryption", "configuration"},
		"kind: EncryptionConfiguration "+secretSentinel,
	)

	assert.Equal(t, "(hidden)", got)
	assert.Nil(t, diffs.MaskSensitive([]string{"spec", "plugins", "apiToken"}, nil))
}

func TestMaskSensitive_URLUserinfo(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		value string
		want  string
	}{
		{"a user and a password", "http://user:" + secretSentinel + "@proxy.example.test:3128", "http://(hidden)@proxy.example.test:3128"},
		{"a token as user", "https://" + secretSentinel + "@charts.example.test/stable?x=1", "https://(hidden)@charts.example.test/stable?x=1"},
		{"an at sign in the path", "https://charts.example.test/a@b", "https://charts.example.test/a@b"},
		{"a URL without credentials", "https://charts.example.test/stable", "https://charts.example.test/stable"},
		{"an email address", "admin@example.test", "admin@example.test"},
		{"a text that is not a URL", "loki", "loki"},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			path := []string{"spec", "kubernetes", "proxy", "http"}

			assert.Equal(t, tC.want, diffs.MaskSensitive(path, tC.value))
			assert.Equal(
				t,
				map[string]any{"url": tC.want},
				diffs.MaskSensitive([]string{"spec", "plugins", "helm", "repositories", "0"}, map[string]any{"url": tC.value}),
			)
		})
	}
}

// The fixture lists fields of the public schemas of the distribution: the ones that hold
// a credential must be hidden, the ones whose key only looks sensitive must be shown.
func TestIsSensitivePath_SchemaFields(t *testing.T) {
	t.Parallel()

	fields, err := yamlx.FromFileV3[struct {
		Hidden []string `yaml:"hidden"`
		Shown  []string `yaml:"shown"`
	}](filepath.Join("testdata", "sensitive", "schema-fields.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, fields.Hidden)
	require.NotEmpty(t, fields.Shown)

	for _, field := range fields.Hidden {
		assert.True(t, diffs.IsSensitivePath(strings.Split(field, ".")), "must be hidden: %s", field)
	}

	for _, field := range fields.Shown {
		assert.False(t, diffs.IsSensitivePath(strings.Split(field, ".")), "must be shown: %s", field)
	}
}
