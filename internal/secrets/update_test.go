// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package secrets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/secrets"
)

func TestUpdateConfigKeepsTheCommentsAndTheFormat(t *testing.T) {
	t.Parallel()

	config := `---
apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
spec:
  # The version of the distribution
  distributionVersion: v1.32.1

  distribution:
    modules:
      # This section holds the configuration of the auth module
      auth:
        provider:
          type: sso
        baseDomain: example.dev

      logging:
        type: loki
        minio:
          rootUser:
            username: minio
            password: ""   # keep this comment

  # Plugins to install
  #plugins: {}
`

	want := `---
apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
spec:
  # The version of the distribution
  distributionVersion: v1.32.1

  distribution:
    modules:
      # This section holds the configuration of the auth module
      auth:
        provider:
          type: sso
        baseDomain: example.dev
        pomerium:
          secrets:
            COOKIE_SECRET: "{file://./secrets/pomerium/COOKIE_SECRET}"

      logging:
        type: loki
        minio:
          rootUser:
            username: minio
            password: "{file://./secrets/minio-logging/password}"   # keep this comment

  # Plugins to install
  #plugins: {}
`

	path := write(t, config)

	written, kept, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
		result(path, "spec.distribution.modules.logging.minio.rootUser.password", "minio-logging", "password"),
	})

	require.NoError(t, err)
	assert.Equal(t, 2, written)
	assert.Empty(t, kept)
	assert.Equal(t, want, read(t, path))
}

func TestUpdateConfigWritesUnderAFieldWithNoValue(t *testing.T) {
	t.Parallel()

	path := write(t, `spec:
  distribution:
    modules:
      auth:
        pomerium:
        # a comment of the auth module
`)

	written, _, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, written)
	assert.Equal(t, `spec:
  distribution:
    modules:
      auth:
        pomerium:
          secrets:
            COOKIE_SECRET: "{file://./secrets/pomerium/COOKIE_SECRET}"
        # a comment of the auth module
`, read(t, path))
}

func TestUpdateConfigKeepsAFieldThatHoldsAValue(t *testing.T) {
	t.Parallel()

	config := `spec:
  distribution:
    modules:
      auth:
        pomerium:
          secrets:
            COOKIE_SECRET: "{env://COOKIE_SECRET}"
`
	path := write(t, config)

	written, kept, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.NoError(t, err)
	assert.Equal(t, 0, written)
	assert.Equal(t, []string{"spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET"}, kept)
	assert.Equal(t, config, read(t, path), "furyctl changed the configuration file")
}

func TestUpdateConfigChangesNothingTheSecondTime(t *testing.T) {
	t.Parallel()

	path := write(t, `spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
`)

	results := []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
		result(path, "spec.distribution.modules.auth.pomerium.secrets.SHARED_SECRET", "pomerium", "SHARED_SECRET"),
	}

	written, _, err := secrets.UpdateConfig(path, results)
	require.NoError(t, err)
	require.Equal(t, 2, written)

	first := read(t, path)

	written, kept, err := secrets.UpdateConfig(path, results)
	require.NoError(t, err)
	assert.Equal(t, 0, written, "furyctl wrote the fields again")
	assert.Empty(t, kept, "furyctl reports the fields that it wrote itself")
	assert.Equal(t, first, read(t, path))
}

func TestReference(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()

	tests := []struct {
		desc      string
		file      string
		configDir string
		want      string
	}{
		{
			desc:      "a folder inside the folder of the configuration file",
			file:      filepath.Join(folder, "secrets", "pomerium", "COOKIE_SECRET"),
			configDir: folder,
			want:      "{file://./secrets/pomerium/COOKIE_SECRET}",
		},
		{
			desc:      "a folder next to the folder of the configuration file",
			file:      filepath.Join(folder, "secrets", "pomerium", "COOKIE_SECRET"),
			configDir: filepath.Join(folder, "clusters"),
			want:      "{file://../secrets/pomerium/COOKIE_SECRET}",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, secrets.Reference(test.file, test.configDir))
		})
	}
}

func TestSnippet(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	path := filepath.Join(folder, "furyctl.yaml")

	snippet, err := secrets.Snippet([]secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
		result(path, "spec.kubernetes.loadBalancers.keepalived.passphrase", "keepalived", "passphrase"),
	}, folder)

	require.NoError(t, err)
	assert.Equal(t, `spec:
  distribution:
    modules:
      auth:
        pomerium:
          secrets:
            COOKIE_SECRET: '{file://./secrets/pomerium/COOKIE_SECRET}'
  kubernetes:
    loadBalancers:
      keepalived:
        passphrase: '{file://./secrets/keepalived/passphrase}'
`, snippet)
}

// TestUpdateConfigWritesAfterABlockScalar keeps the new fields out of the text of a block scalar.
// The parser gives no line for that text, thus the lines of it are the ones that a naive insertion
// writes into.
func TestUpdateConfigWritesAfterABlockScalar(t *testing.T) {
	t.Parallel()

	path := write(t, `spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
        oidcTrustedCA: |
          -----BEGIN CERTIFICATE-----
          MIIBkTCB+wIJAKl

          -----END CERTIFICATE-----
`)

	written, _, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, written)
	assert.Equal(t, `spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
        oidcTrustedCA: |
          -----BEGIN CERTIFICATE-----
          MIIBkTCB+wIJAKl

          -----END CERTIFICATE-----
        pomerium:
          secrets:
            COOKIE_SECRET: "{file://./secrets/pomerium/COOKIE_SECRET}"
`, read(t, path))

	// The file must still be a configuration file that furyctl can read.
	_, err = secrets.LoadConfig(path)
	require.NoError(t, err)
}

// TestUpdateConfigKeepsAFlowMapping keeps a value that furyctl cannot add a field to.
func TestUpdateConfigKeepsAFlowMapping(t *testing.T) {
	t.Parallel()

	config := `spec:
  distribution:
    modules:
      auth: {}
`
	path := write(t, config)

	written, kept, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.NoError(t, err)
	assert.Equal(t, 0, written)
	assert.Len(t, kept, 1)
	assert.Equal(t, config, read(t, path), "furyctl changed the configuration file")
}

// TestUpdateConfigWritesInAFieldWithACommentAndNoValue keeps the comment of a field that has no
// value at all, and puts a space before it.
func TestUpdateConfigWritesInAFieldWithACommentAndNoValue(t *testing.T) {
	t.Parallel()

	path := write(t, `spec:
  distribution:
    modules:
      auth:
        pomerium:
          secrets:
            COOKIE_SECRET: # write me
`)

	written, _, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, written)
	assert.Equal(t, `spec:
  distribution:
    modules:
      auth:
        pomerium:
          secrets:
            COOKIE_SECRET: "{file://./secrets/pomerium/COOKIE_SECRET}" # write me
`, read(t, path))
}

// TestUpdateConfigReportsTheFileWithNoFields keeps a file that holds no fields as it is, and names
// it in the error.
func TestUpdateConfigReportsTheFileWithNoFields(t *testing.T) {
	t.Parallel()

	path := write(t, "# only a comment\n")

	_, _, err := secrets.UpdateConfig(path, []secrets.Result{
		result(path, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.ErrorIs(t, err, secrets.ErrConfigHoldsNoFields)
	assert.Equal(t, "# only a comment\n", read(t, path), "furyctl changed the configuration file")
}

// TestUpdateConfigKeepsASymbolicLink writes the file at the end of the link, because a
// configuration file can be a link to a file that more clusters share.
func TestUpdateConfigKeepsASymbolicLink(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	target := filepath.Join(folder, "cluster.yaml")
	link := filepath.Join(folder, "furyctl.yaml")

	require.NoError(t, os.WriteFile(target, []byte(`spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
`), 0o600))
	require.NoError(t, os.Symlink(target, link))

	written, _, err := secrets.UpdateConfig(link, []secrets.Result{
		result(link, "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET", "pomerium", "COOKIE_SECRET"),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, written)
	assert.Contains(t, read(t, target), "COOKIE_SECRET: \"{file://./secrets/pomerium/COOKIE_SECRET}\"")

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "furyctl replaced the link with a file")
}

// result returns the secret of a component in the `secrets` folder next to the configuration file.
func result(configPath, path, component, name string) secrets.Result {
	return secrets.Result{
		Path:    path,
		File:    filepath.Join(filepath.Dir(configPath), "secrets", component, name),
		Created: true,
	}
}

func write(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
