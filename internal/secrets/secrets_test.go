// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package secrets_test

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/secrets"
)

// keyBytes is the number of bytes of the secrets that Pomerium and Gangplank read as a 256-bit key.
const keyBytes = 32

// safeValue matches the values that furyctl writes with no encoding. Such a value needs no quotes
// and no escape in a YAML file, in a URL, and in an environment file.
var safeValue = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func TestWriteCreatesOneFileForEachSecret(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()

	results, err := secrets.Write(secrets.All(), folder)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	for _, result := range results {
		assert.True(t, result.Created, "furyctl did not create %s", result.File)
		assert.Equal(t, folder, filepath.Dir(filepath.Dir(result.File)), "%s is in the wrong folder", result.File)

		info, err := os.Stat(result.File)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the file %s is readable by others", result.File)
	}
}

func TestWriteKeepsTheFilesThatAreAlreadyThere(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	components := secrets.All()

	first, err := secrets.Write(components, folder)
	require.NoError(t, err)

	before := map[string]string{}

	for _, result := range first {
		before[result.File] = read(t, result.File)
	}

	second, err := secrets.Write(components, folder)
	require.NoError(t, err)
	require.Len(t, second, len(first))

	for _, result := range second {
		assert.False(t, result.Created, "furyctl wrote %s again", result.File)
		assert.Equal(t, before[result.File], read(t, result.File), "the value of %s changed", result.File)
	}
}

// TestWriteKeepsThePermissionsOfAFileThatIsAlreadyThere pins the behavior that the warning about
// the open files rests on: furyctl reports such a file, and changes neither its value nor its
// permissions. A clone of a repository holds each file with 0644, because git writes the files of
// the index with the permissions of the umask.
func TestWriteKeepsThePermissionsOfAFileThatIsAlreadyThere(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	components, err := secrets.Select([]string{"pomerium"}, nil)
	require.NoError(t, err)

	first, err := secrets.Write(components, folder)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	file := first[0].File
	require.NoError(t, os.Chmod(file, 0o644))

	before := read(t, file)

	second, err := secrets.Write(components, folder)
	require.NoError(t, err)

	assert.False(t, second[0].Created, "furyctl wrote the file again")
	assert.Equal(t, before, read(t, file), "the value changed")

	info, err := os.Stat(file)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "furyctl changed the permissions")
}

// TestSecretValues keeps the values that furyctl creates in the format that each package reads.
func TestSecretValues(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()

	results, err := secrets.Write(secrets.All(), folder)
	require.NoError(t, err)

	values := map[string]bool{}

	for _, result := range results {
		value := strings.TrimSuffix(read(t, result.File), "\n")
		name := filepath.Base(result.File)

		require.NotEmpty(t, value, "the value of %s is empty", result.Path)
		assert.False(t, values[value], "two secrets hold the same value: %s", result.Path)
		values[value] = true

		switch name {
		case "COOKIE_SECRET", "SHARED_SECRET", "sessionSecurityKey":
			raw, err := base64.StdEncoding.DecodeString(value)
			require.NoError(t, err, "%s is not in base64", result.Path)
			assert.Len(t, raw, keyBytes, "%s is not a 256-bit key", result.Path)

		case "SIGNING_KEY":
			raw, err := base64.StdEncoding.DecodeString(value)
			require.NoError(t, err, "%s is not in base64", result.Path)

			block, _ := pem.Decode(raw)
			require.NotNil(t, block, "%s is not in PEM", result.Path)
			assert.Equal(t, "EC PRIVATE KEY", block.Type)

			key, err := x509.ParseECPrivateKey(block.Bytes)
			require.NoError(t, err, "%s is not an EC private key", result.Path)
			assert.Equal(t, "P-256", key.Curve.Params().Name)

		case "configuration":
			assertEncryptionConfiguration(t, value)

		case "passphrase":
			// The schema of the cluster rejects a passphrase of more than 8 characters.
			assert.LessOrEqual(t, len(value), 8)
			assert.Regexp(t, safeValue, value)

		default:
			assert.Regexp(t, safeValue, value)
			assert.GreaterOrEqual(t, len(value), 16, "the value of %s is too short", result.Path)
		}
	}
}

// TestWriteLeavesNoFileWhenTheValueFails keeps an empty file out of the folder: the O_EXCL flag of
// the run after this one would keep that file, and the configuration file would read no value.
func TestWriteLeavesNoFileWhenTheValueFails(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	broken := errors.New("no random bytes")

	_, err := secrets.Write([]secrets.Component{{
		Name: "pomerium",
		Secrets: []secrets.Secret{{
			Name: "COOKIE_SECRET",
			Path: "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET",
			New:  func() (string, error) { return "", broken },
		}},
	}}, folder)

	require.ErrorIs(t, err, broken)

	_, err = os.Stat(filepath.Join(folder, "pomerium", "COOKIE_SECRET"))
	require.ErrorIs(t, err, os.ErrNotExist, "furyctl left an empty file behind")
}

// assertEncryptionConfiguration keeps the object that the API server reads: a secretbox provider
// with a 256-bit key, and the identity provider last. Without the identity provider the API server
// cannot read the secrets that etcd holds with no encryption.
func assertEncryptionConfiguration(t *testing.T, value string) {
	t.Helper()

	var config struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Resources  []struct {
			Resources []string `yaml:"resources"`
			Providers []struct {
				Secretbox *struct {
					Keys []struct {
						Name   string `yaml:"name"`
						Secret string `yaml:"secret"`
					} `yaml:"keys"`
				} `yaml:"secretbox"`
				Identity map[string]any `yaml:"identity"`
			} `yaml:"providers"`
		} `yaml:"resources"`
	}

	require.NoError(t, yaml.Unmarshal([]byte(value), &config), "the configuration is not valid YAML")
	assert.Equal(t, "apiserver.config.k8s.io/v1", config.APIVersion)
	assert.Equal(t, "EncryptionConfiguration", config.Kind)

	require.Len(t, config.Resources, 1)
	assert.Equal(t, []string{"secrets"}, config.Resources[0].Resources)

	providers := config.Resources[0].Providers
	require.Len(t, providers, 2)
	require.NotNil(t, providers[0].Secretbox, "the first provider is not secretbox")
	require.Len(t, providers[0].Secretbox.Keys, 1)

	raw, err := base64.StdEncoding.DecodeString(providers[0].Secretbox.Keys[0].Secret)
	require.NoError(t, err, "the key of secretbox is not in base64")
	assert.Len(t, raw, keyBytes, "the key of secretbox is not a 256-bit key")

	assert.Nil(t, providers[1].Secretbox, "the last provider is not identity")
	assert.NotNil(t, providers[1].Identity, "the last provider is not identity")
}

func TestEncryption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc      string
		config    string
		supported bool
	}{
		{
			desc: "an on-premises cluster with an empty field",
			config: `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: ""`,
			supported: true,
		},
		{
			desc:      "an on-premises cluster with no field",
			config:    "kind: OnPremises",
			supported: true,
		},
		{
			desc: "a field that holds a value already",
			config: `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: |
          apiVersion: apiserver.config.k8s.io/v1`,
			supported: false,
		},
		{
			desc:      "a kind with no control plane to configure",
			config:    "kind: EKSCluster",
			supported: false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "furyctl.yaml")
			require.NoError(t, os.WriteFile(path, []byte(test.config), 0o600))

			cfg, err := secrets.LoadConfig(path)
			require.NoError(t, err)

			// A nil schema is the answer for a cluster whose schema furyctl cannot read: the
			// kinds that have had the field decide. TestEncryptionReadsTheSchema covers the rest.
			component, supported := secrets.Encryption(cfg, nil)
			assert.Equal(t, test.supported, supported)
			assert.Equal(t, secrets.EncryptionComponent, component.Name)
		})
	}
}

// TestEncryptionNeedsTheUser keeps the encryption at rest configuration out of a run that names no
// component. Only the user can ask for it, thus neither a configuration file nor the absence of one
// selects it.
func TestEncryptionNeedsTheUser(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte("kind: OnPremises"), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	tests := []struct {
		desc string
		cfg  *secrets.Config
	}{
		{desc: "with a configuration file", cfg: cfg},
		{desc: "with no configuration file", cfg: nil},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			selected, err := secrets.Select(nil, test.cfg)
			require.NoError(t, err)
			assert.NotContains(t, names(selected), secrets.EncryptionComponent)
		})
	}

	// The name on the command line is the answer of the user, thus it selects the component.
	named, err := secrets.Select([]string{secrets.EncryptionComponent}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{secrets.EncryptionComponent}, names(named))
}

func TestSelectRemovesTheNamesThatAreThereTwice(t *testing.T) {
	t.Parallel()

	selected, err := secrets.Select([]string{"pomerium", "keepalived", "pomerium"}, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"pomerium", "keepalived"}, names(selected))
}

func TestSelect(t *testing.T) {
	t.Parallel()

	// A run with no configuration file creates the secrets of each component that a configuration
	// file can need. The encryption at rest is not one of them: only the user can ask for it.
	automatic := lo.Filter(names(secrets.All()), func(name string, _ int) bool {
		return name != secrets.EncryptionComponent
	})

	tests := []struct {
		desc   string
		args   []string
		config string
		want   []string
	}{
		{
			desc: "no configuration file creates the secrets that a configuration can need",
			want: automatic,
		},
		{
			desc: "the names on the command line win over the configuration file",
			args: []string{"pomerium", "keepalived"},
			config: `spec:
  distribution:
    modules:
      auth:
        provider:
          type: none`,
			want: []string{"pomerium", "keepalived"},
		},
		{
			desc: "the sso provider needs Pomerium",
			config: `spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
      logging:
        type: none
      monitoring:
        type: none
      tracing:
        type: none`,
			want: []string{"pomerium"},
		},
		{
			desc: "the basicAuth provider needs a password",
			config: `spec:
  distribution:
    modules:
      auth:
        provider:
          type: basicAuth
        oidcKubernetesAuth:
          enabled: true
      logging:
        type: none
      monitoring:
        type: none
      tracing:
        type: none`,
			want: []string{"gangplank", "basic-auth"},
		},
		{
			desc: "the default modules deploy MinIO for logging and tracing",
			config: `spec:
  distribution:
    modules:
      auth:
        provider:
          type: none`,
			want: []string{"minio-logging", "minio-tracing"},
		},
		{
			desc: "Mimir with an external bucket needs no MinIO",
			config: `spec:
  distribution:
    modules:
      auth:
        provider:
          type: none
      logging:
        type: customOutputs
      monitoring:
        type: mimir
        mimir:
          backend: externalEndpoint
      tracing:
        type: tempo
        tempo:
          backend: externalEndpoint`,
			want: []string{},
		},
		{
			desc: "an immutable cluster with both Keepalived clusters needs two passphrases",
			config: `kind: Immutable
spec:
  infrastructure:
    loadBalancers:
      keepalived:
        enabled: true
  kubernetes:
    controlPlane:
      keepalived:
        enabled: true
  distribution:
    modules:
      auth:
        provider:
          type: none
      logging:
        type: none
      monitoring:
        type: none
      tracing:
        type: none`,
			want: []string{"keepalived", "keepalived-controlplane"},
		},
		{
			desc: "the load balancers need a passphrase and a password",
			config: `spec:
  kubernetes:
    loadBalancers:
      enabled: true
      keepalived:
        enabled: true
  distribution:
    modules:
      auth:
        provider:
          type: none
      logging:
        type: none
      monitoring:
        type: none
      tracing:
        type: none`,
			want: []string{"keepalived", "haproxy-stats"},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			var cfg *secrets.Config

			if test.config != "" {
				path := filepath.Join(t.TempDir(), "furyctl.yaml")
				require.NoError(t, os.WriteFile(path, []byte(test.config), 0o600))

				loaded, err := secrets.LoadConfig(path)
				require.NoError(t, err)

				cfg = loaded
			}

			selected, err := secrets.Select(test.args, cfg)
			require.NoError(t, err)
			assert.Equal(t, test.want, names(selected))
		})
	}
}

// TestKeepalivedFieldPerKind keeps the field of the passphrase together with the kind. The
// Immutable kind configures the load balancers in the infrastructure phase, thus its field is not
// the field of the other kinds.
func TestKeepalivedFieldPerKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		config string
		want   string
	}{
		{
			desc: "an on-premises cluster",
			config: `kind: OnPremises
spec:
  kubernetes:
    loadBalancers:
      enabled: true
      keepalived:
        enabled: true`,
			want: "spec.kubernetes.loadBalancers.keepalived.passphrase",
		},
		{
			desc: "an immutable cluster",
			config: `kind: Immutable
spec:
  infrastructure:
    loadBalancers:
      keepalived:
        enabled: true`,
			want: "spec.infrastructure.loadBalancers.keepalived.passphrase",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "furyctl.yaml")
			require.NoError(t, os.WriteFile(path, []byte(test.config), 0o600))

			cfg, err := secrets.LoadConfig(path)
			require.NoError(t, err)

			selected, err := secrets.Select(nil, cfg)
			require.NoError(t, err)
			require.Contains(t, names(selected), "keepalived", "the cluster needs a passphrase")

			component, found := lo.Find(selected, func(c secrets.Component) bool { return c.Name == "keepalived" })
			require.True(t, found)
			require.Len(t, component.Secrets, 1)
			assert.Equal(t, test.want, component.Secrets[0].Path)
		})
	}
}

func TestSelectRejectsAnUnknownComponent(t *testing.T) {
	t.Parallel()

	_, err := secrets.Select([]string{"pomerium", "nginx"}, nil)

	require.ErrorIs(t, err, secrets.ErrUnknownComponent)
	assert.Contains(t, err.Error(), "pomerium", "the message does not list the components")
}

func names(components []secrets.Component) []string {
	return lo.Map(components, func(c secrets.Component, _ int) string { return c.Name })
}

func read(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

// TestWriteRefusesAPathThatHoldsNoSecret covers the paths that the O_EXCL flag reports as already
// there, and that furyctl cannot read a value from. Without this test the report says that furyctl
// kept a secret, and the configuration file reads an empty value from the path.
func TestWriteRefusesAPathThatHoldsNoSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		build func(t *testing.T, file string)
	}{
		{
			desc: "a file with no value in it",
			build: func(t *testing.T, file string) {
				t.Helper()
				require.NoError(t, os.WriteFile(file, []byte("\n  \n"), 0o600))
			},
		},
		{
			desc: "a folder with the name of the secret",
			build: func(t *testing.T, file string) {
				t.Helper()
				require.NoError(t, os.Mkdir(file, 0o700))
			},
		},
		{
			// The `{file://}` notation reads the file with furyctl, thus a file that furyctl cannot
			// read holds no value for the cluster: see keepsASecret.
			desc: "a file that furyctl cannot read",
			build: func(t *testing.T, file string) {
				t.Helper()
				require.NoError(t, os.WriteFile(file, []byte("a value"), 0o600))
				require.NoError(t, os.Chmod(file, 0o000))

				// An account with no limits reads such a file, and the test then has nothing to
				// report. The pipelines run as one of those accounts.
				if _, err := os.ReadFile(file); err == nil {
					t.Skip("this account reads a file with no permissions for it")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			folder := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(folder, "basic-auth"), 0o700))
			test.build(t, filepath.Join(folder, "basic-auth", "password"))

			components, err := secrets.Select([]string{"basic-auth"}, nil)
			require.NoError(t, err)

			_, err = secrets.Write(components, folder)
			require.ErrorIs(t, err, secrets.ErrFileHoldsNoSecret)
		})
	}
}

// TestSelectExpandsDynamicValues reads the fields that hold a `{env://}` or a `{file://}` value.
// The configuration file gives such a value to any field, thus a cluster whose authentication type
// comes from the environment needs the secrets of that type. This test sets a variable of the
// environment, thus neither it nor its subtests can run in parallel.
func TestSelectExpandsDynamicValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	config := `kind: OnPremises
spec:
  distribution:
    modules:
      auth:
        provider:
          type: "{env://AUTH_TYPE}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	tests := []struct {
		desc       string
		value      string
		components []string
	}{
		{"a value that names a type", "sso", []string{"pomerium"}},
		{"another value that names a type", "basicAuth", []string{"basic-auth"}},
		{"a variable with no value", "", nil},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Setenv("AUTH_TYPE", test.value)

			cfg, err := secrets.LoadConfig(path)
			require.NoError(t, err)

			components, err := secrets.Select(nil, cfg)
			require.NoError(t, err)

			names := lo.Map(components, func(c secrets.Component, _ int) string { return c.Name })

			for _, name := range test.components {
				assert.Contains(t, names, name)
			}

			if test.components == nil {
				assert.NotContains(t, names, "pomerium")
				assert.NotContains(t, names, "basic-auth")

				// The value of the field is unknown, thus furyctl names the field and the command
				// stops. An empty value is an answer, and a wrong one.
				assert.Equal(t, []string{"spec.distribution.modules.auth.provider.type"}, cfg.Unresolved())

				return
			}

			assert.Empty(t, cfg.Unresolved())
		})
	}
}

// TestEncryptionKeepsAReferenceThatFuryctlCannotRead is the guard against a second key. The
// encryption field holds the configuration of etcd, and furyctl writes it as a `{file://}`
// reference. A user who keeps the secrets out of the repository, as the command asks, has no such
// file on another machine. An empty answer there would make furyctl write a new key over the key
// that etcd already holds, and the secrets of the cluster would stay unreadable.
func TestEncryptionKeepsAReferenceThatFuryctlCannotRead(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	path := filepath.Join(folder, "furyctl.yaml")
	config := `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: "{file://./secrets/etcd-encryption/configuration}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	_, supported := secrets.Encryption(cfg, nil)
	assert.False(t, supported, "furyctl offered to write a second key")

	assert.Equal(t,
		[]string{"spec.kubernetes.advanced.encryption.configuration"},
		cfg.Unresolved(),
		"furyctl did not name the field that it cannot read",
	)
}

// TestEncryptionKeepsAReferenceToAFileWithNoValue covers the other half of the guard against a
// second key. A file that is there and holds no value resolves with no error, thus the field looks
// like a cluster that holds no encryption at rest. Such a file comes from a run that furyctl could
// not finish: see writeNew.
func TestEncryptionKeepsAReferenceToAFileWithNoValue(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	path := filepath.Join(folder, "furyctl.yaml")
	config := `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: "{file://./configuration}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(folder, "configuration"), []byte("\n"), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	_, supported := secrets.Encryption(cfg, nil)
	assert.False(t, supported, "furyctl offered to write a second key")

	assert.Equal(t,
		[]string{"spec.kubernetes.advanced.encryption.configuration"},
		cfg.Unresolved(),
		"furyctl did not name the field that holds no value",
	)
}

// TestWriteGoesOnAfterAPathThatHoldsNoSecret keeps the secrets of a run that meets one bad path. A
// stop would leave those files on the disk with no field that reads them, and the run after this
// one would stop at the same path.
func TestWriteGoesOnAfterAPathThatHoldsNoSecret(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()

	// The basic-auth component comes before keepalived in All, thus the bad path is not the last one.
	require.NoError(t, os.Mkdir(filepath.Join(folder, "basic-auth"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(folder, "basic-auth", "password"), nil, 0o600))

	components, err := secrets.Select([]string{"pomerium", "basic-auth", "keepalived"}, nil)
	require.NoError(t, err)

	results, err := secrets.Write(components, folder)

	require.ErrorIs(t, err, secrets.ErrFileHoldsNoSecret)
	require.NotEmpty(t, results, "furyctl kept no result, thus nothing references the files it wrote")

	names := lo.Map(results, func(r secrets.Result, _ int) string { return filepath.Base(r.File) })
	assert.Contains(t, names, "COOKIE_SECRET")
	assert.Contains(t, names, "passphrase")
	assert.NotContains(t, names, "password", "furyctl reported a path that holds no secret")

	for _, result := range results {
		assert.FileExists(t, result.File)
	}
}

// TestWriteGoesOnAfterAFolderThatItCannotMake keeps the secrets of a run that meets a component
// folder it cannot make. A stop leaves those files with no reference to them, and with no warning
// about the private keys in them.
func TestWriteGoesOnAfterAFolderThatItCannotMake(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()

	// A file with the name of the component folder: MkdirAll cannot make a folder over it.
	require.NoError(t, os.WriteFile(filepath.Join(folder, "keepalived"), nil, 0o600))

	components, err := secrets.Select([]string{"pomerium", "keepalived"}, nil)
	require.NoError(t, err)

	results, err := secrets.Write(components, folder)

	require.Error(t, err)
	require.NotEmpty(t, results, "furyctl kept no result, thus nothing references the files it wrote")

	names := lo.Map(results, func(r secrets.Result, _ int) string { return filepath.Base(r.File) })
	assert.Contains(t, names, "SIGNING_KEY")
	assert.NotContains(t, names, "passphrase")

	for _, result := range results {
		assert.FileExists(t, result.File)
	}
}

// TestBoolNamesAFieldThatHoldsNoBoolean covers the fields that decide whether a cluster uses a
// component. The schema of a cluster asks for a boolean in each of them and takes no text: yaml.v3
// follows YAML 1.2, thus `off` and `"true"` are both text and `furyctl validate config` reports
// both. A field of that shape gets a name from furyctl instead of a guess. Without that report
// furyctl leaves the component out and says nothing, and the cluster starts without its secrets.
func TestBoolNamesAFieldThatHoldsNoBoolean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		value    string
		selected bool
		named    bool
	}{
		{"a boolean of true", "true", true, false},
		{"a boolean of false", "false", false, false},
		{"no field at all", "", false, false},
		{"the text of a boolean", `"true"`, false, true},
		{"a spelling of YAML 1.1", "off", false, true},
		{"a value from somewhere else", `"{env://FURYCTL_UNSET_ON_PURPOSE}"`, false, true},
		{"a number", "1", false, true},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			config := "kind: OnPremises"
			if test.value != "" {
				config += `
spec:
  kubernetes:
    loadBalancers:
      keepalived:
        enabled: ` + test.value
			}

			path := filepath.Join(t.TempDir(), "furyctl.yaml")
			require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

			cfg, err := secrets.LoadConfig(path)
			require.NoError(t, err)

			components, err := secrets.Select(nil, cfg)
			require.NoError(t, err)

			names := lo.Map(components, func(c secrets.Component, _ int) string { return c.Name })

			if test.selected {
				assert.Contains(t, names, "keepalived")
			} else {
				assert.NotContains(t, names, "keepalived")
			}

			if test.named {
				assert.Equal(t,
					[]string{"spec.kubernetes.loadBalancers.keepalived.enabled"},
					cfg.Unresolved(),
					"furyctl left the component out and named no field",
				)

				return
			}

			assert.Empty(t, cfg.Unresolved())
		})
	}
}

// TestUnreadableCoversEachSecretThatFuryctlWrites is the guard against a second secret, on the path
// that names a component. Encryption reads the encryption field, and the command calls Encryption
// only for a run that names nothing: a run that names the component must reach the same guard.
func TestUnreadableCoversEachSecretThatFuryctlWrites(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	path := filepath.Join(folder, "furyctl.yaml")
	config := `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: "{file://./secrets/etcd-encryption/configuration}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	// The name on the command line, as `furyctl create secrets etcd-encryption` gives it.
	components, err := secrets.Select([]string{secrets.EncryptionComponent}, cfg)
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"spec.kubernetes.advanced.encryption.configuration"},
		secrets.Unreadable(components, cfg, nil, nil),
		"furyctl would write a second key over a reference that it cannot follow",
	)
}

// TestUnreadableTakesAConfigurationThatFuryctlCanRead keeps a run alive when each field of each
// secret holds a value that furyctl can read, or no value at all.
func TestUnreadableTakesAConfigurationThatFuryctlCanRead(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	path := filepath.Join(folder, "furyctl.yaml")
	config := `kind: OnPremises
spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
        pomerium:
          secrets:
            COOKIE_SECRET: "{file://./cookie}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(folder, "cookie"), []byte("a value"), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	components, err := secrets.Select(nil, cfg)
	require.NoError(t, err)

	assert.Empty(t, secrets.Unreadable(components, cfg, nil, nil))
}

// TestStringNamesAValueThatIsNotText covers the last way a field reaches a decision. Config.String
// and Config.Bool are the two ways, and Bool already names a value of the wrong kind. A field that
// holds a map looks like an empty field without this report, and an empty encryption field is the
// answer that makes furyctl write a second key.
func TestStringNamesAValueThatIsNotText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		value string
		named bool
	}{
		{"a block scalar, as the schema asks for", "|\n          apiVersion: v1", false},
		{"a map, as a user who forgets the block scalar writes", "\n          apiVersion: v1", true},
		{"a number", "42", true},
		{"a boolean", "true", true},
		{"no value at all", "\"\"", false},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "furyctl.yaml")
			config := `kind: OnPremises
spec:
  kubernetes:
    advanced:
      encryption:
        configuration: ` + test.value

			require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

			cfg, err := secrets.LoadConfig(path)
			require.NoError(t, err)

			_, supported := secrets.Encryption(cfg, nil)

			if test.named {
				assert.False(t, supported, "furyctl offered to write a second key")
				assert.Equal(t,
					[]string{"spec.kubernetes.advanced.encryption.configuration"},
					cfg.Unresolved(),
					"furyctl did not name the field that holds no text",
				)

				return
			}

			assert.Empty(t, cfg.Unresolved())
		})
	}
}

// TestUnreadableWithNoConfiguration keeps the answer of a run with no configuration file. Config
// holds the fields that furyctl reads, thus a run with no file has none to read.
func TestUnreadableWithNoConfiguration(t *testing.T) {
	t.Parallel()

	assert.Nil(t, secrets.Unreadable(secrets.All(), nil, nil, nil))
}

// TestHeldLeavesASecretThatTheClusterHoldsSomewhereElse keeps the value that a cluster runs with. A
// field that holds a value names the secret of the cluster. Without the file of that secret furyctl
// would write a new value, and the cluster would keep the first one: two secrets, and no way to tell
// which of them a service reads.
func TestHeldLeavesASecretThatTheClusterHoldsSomewhereElse(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	path := filepath.Join(folder, "furyctl.yaml")
	config := `kind: OnPremises
spec:
  distribution:
    modules:
      auth:
        provider:
          type: sso
        pomerium:
          secrets:
            SHARED_SECRET: a-value-the-cluster-runs-with`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	components, err := secrets.Select([]string{"pomerium"}, cfg)
	require.NoError(t, err)

	secretsFolder := filepath.Join(folder, "secrets")
	kept, held := secrets.Held(components, cfg, secretsFolder)

	assert.Equal(t,
		[]string{"spec.distribution.modules.auth.pomerium.secrets.SHARED_SECRET"},
		held,
		"furyctl would write a second secret",
	)

	require.Len(t, kept, 1)

	names := lo.Map(kept[0].Secrets, func(s secrets.Secret, _ int) string { return s.Name })
	assert.NotContains(t, names, "SHARED_SECRET")
	assert.Contains(t, names, "COOKIE_SECRET")

	// The file of that secret makes the field and the file the same secret, thus furyctl keeps the
	// file and Write reports it as one that was already there.
	require.NoError(t, os.MkdirAll(filepath.Join(secretsFolder, "pomerium"), 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(secretsFolder, "pomerium", "SHARED_SECRET"), []byte("a value"), 0o600))

	_, held = secrets.Held(components, cfg, secretsFolder)
	assert.Empty(t, held)
}

// TestUnreadableLeavesTheFieldsThatDecideToARunThatNamesNothing keeps a named run out of the fields
// that decide. Select reads those fields only for a run that names nothing, thus a named run must
// not stop over one of them: `furyctl create secrets pomerium` says what to create, and a field of
// another component decides nothing about it.
func TestUnreadableLeavesTheFieldsThatDecideToARunThatNamesNothing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")
	config := `kind: OnPremises
spec:
  kubernetes:
    loadBalancers:
      enabled: "{env://FURYCTL_UNSET_ON_PURPOSE}"`

	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))

	cfg, err := secrets.LoadConfig(path)
	require.NoError(t, err)

	components, err := secrets.Select([]string{"pomerium"}, cfg)
	require.NoError(t, err)

	assert.Empty(t, secrets.Unreadable(components, cfg, nil, []string{"pomerium"}),
		"a named run stopped over a field that decides nothing about it")

	// A run that names nothing reads that field, thus it names it.
	assert.Equal(t,
		[]string{"spec.kubernetes.loadBalancers.enabled"},
		secrets.Unreadable(components, cfg, nil, nil),
	)
}

// TestEncryptionKeyNameIsNotTheSameTwice keeps a new key from taking the name of the key that a
// cluster holds. The API server reads the keys of etcd by name: two keys with one name make it read
// the wrong one, and the secrets that etcd holds stay unreadable.
func TestEncryptionKeyNameIsNotTheSameTwice(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()

	components, err := secrets.Select([]string{secrets.EncryptionComponent}, nil)
	require.NoError(t, err)

	names := map[string]bool{}

	for range 5 {
		require.NoError(t, os.RemoveAll(folder))

		results, err := secrets.Write(components, folder)
		require.NoError(t, err)
		require.Len(t, results, 1)

		name := keyNameOf(t, read(t, results[0].File))

		assert.NotEqual(t, "key1", name, "the name of the key is the same for each cluster")
		assert.False(t, names[name], "two keys hold the name %s", name)
		names[name] = true
	}
}

// keyNameOf returns the name of the first key of an encryption at rest configuration.
func keyNameOf(t *testing.T, value string) string {
	t.Helper()

	var config struct {
		Resources []struct {
			Providers []struct {
				Secretbox *struct {
					Keys []struct {
						Name string `yaml:"name"`
					} `yaml:"keys"`
				} `yaml:"secretbox"`
			} `yaml:"providers"`
		} `yaml:"resources"`
	}

	require.NoError(t, yaml.Unmarshal([]byte(value), &config))
	require.NotEmpty(t, config.Resources)
	require.NotEmpty(t, config.Resources[0].Providers)
	require.NotNil(t, config.Resources[0].Providers[0].Secretbox)
	require.NotEmpty(t, config.Resources[0].Providers[0].Secretbox.Keys)

	return config.Resources[0].Providers[0].Secretbox.Keys[0].Name
}
