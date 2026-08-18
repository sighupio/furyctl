// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package clusterpki_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/clusterpki"
)

// completePKI writes a PKI folder that holds every file the installer roles read.
func completePKI(t *testing.T) string {
	t.Helper()

	base := filepath.Join(t.TempDir(), "pki")

	files := map[string][]string{
		"master": {"ca.crt", "ca.key", "front-proxy-ca.crt", "front-proxy-ca.key", "sa.key", "sa.pub"},
		"etcd":   {"ca.crt", "ca.key"},
	}

	for dir, names := range files {
		require.NoError(t, os.MkdirAll(filepath.Join(base, dir), os.ModePerm))

		for _, name := range names {
			require.NoError(t, os.WriteFile(filepath.Join(base, dir, name), []byte("x"), 0o600))
		}
	}

	return base
}

func TestCheck_Complete(t *testing.T) {
	t.Parallel()

	require.NoError(t, clusterpki.Check(completePKI(t)))
}

func TestCheck_FolderMissing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pki")

	err := clusterpki.Check(path)

	folderMissing := &clusterpki.FolderMissingError{}
	require.ErrorAs(t, err, &folderMissing)
	assert.Equal(t, path, folderMissing.Path)
	assert.Contains(t, err.Error(), "furyctl create pki --path "+path)
}

func TestCheck_PathIsAFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pki")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

	err := clusterpki.Check(path)

	require.ErrorIs(t, err, clusterpki.ErrPathNotAFolder)

	// The folder exists, so the message must not tell the user to create it.
	folderMissing := &clusterpki.FolderMissingError{}
	assert.NotErrorAs(t, err, &folderMissing)
}

// furyctl must not report a file that it cannot read as absent. The message for an absent file tells the
// user to delete the folder, and that step destroys the CA of a cluster that exists.
func TestCheck_FileIsUnreadable(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads a folder without the execute permission")
	}

	base := completePKI(t)
	require.NoError(t, os.Chmod(filepath.Join(base, "master"), 0o000))

	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(base, "master"), 0o700)
	})

	err := clusterpki.Check(base)

	require.Error(t, err)

	filesMissing := &clusterpki.FilesMissingError{}
	assert.NotErrorAs(t, err, &filesMissing)
	assert.ErrorIs(t, err, os.ErrPermission)
}

func TestCheck_FilesMissing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		mutate      func(t *testing.T, base string)
		wantMissing []string
	}{
		{
			desc: "one file of the control plane is absent",
			mutate: func(t *testing.T, base string) {
				t.Helper()
				require.NoError(t, os.Remove(filepath.Join(base, "master", "sa.pub")))
			},
			wantMissing: []string{filepath.Join("master", "sa.pub")},
		},
		{
			desc: "one file is empty",
			mutate: func(t *testing.T, base string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(base, "etcd", "ca.key"), []byte(""), 0o600))
			},
			wantMissing: []string{filepath.Join("etcd", "ca.key")},
		},
		{
			desc: "one file is a folder",
			mutate: func(t *testing.T, base string) {
				t.Helper()
				path := filepath.Join(base, "master", "ca.crt")
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Mkdir(path, os.ModePerm))
			},
			wantMissing: []string{filepath.Join("master", "ca.crt")},
		},
		{
			desc: "the etcd folder is absent",
			mutate: func(t *testing.T, base string) {
				t.Helper()
				require.NoError(t, os.RemoveAll(filepath.Join(base, "etcd")))
			},
			wantMissing: []string{filepath.Join("etcd", "ca.crt"), filepath.Join("etcd", "ca.key")},
		},
		{
			desc: "the folder is empty",
			mutate: func(t *testing.T, base string) {
				t.Helper()
				require.NoError(t, os.RemoveAll(filepath.Join(base, "master")))
				require.NoError(t, os.RemoveAll(filepath.Join(base, "etcd")))
			},
			wantMissing: []string{
				filepath.Join("master", "ca.crt"),
				filepath.Join("master", "ca.key"),
				filepath.Join("master", "front-proxy-ca.crt"),
				filepath.Join("master", "front-proxy-ca.key"),
				filepath.Join("master", "sa.key"),
				filepath.Join("master", "sa.pub"),
				filepath.Join("etcd", "ca.crt"),
				filepath.Join("etcd", "ca.key"),
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			base := completePKI(t)
			tC.mutate(t, base)

			filesMissing := &clusterpki.FilesMissingError{}
			require.ErrorAs(t, clusterpki.Check(base), &filesMissing)
			assert.Equal(t, tC.wantMissing, filesMissing.Missing)
		})
	}
}

func TestResolvePath(t *testing.T) {
	t.Parallel()

	confDir := filepath.Join(string(os.PathSeparator), "home", "user", "lab")

	testCases := []struct {
		desc    string
		confDir string
		value   string
		want    string
		wantErr error
	}{
		{
			desc:  "an absolute path does not change",
			value: filepath.Join(string(os.PathSeparator), "opt", "pki"),
			want:  filepath.Join(string(os.PathSeparator), "opt", "pki"),
		},
		{
			desc:  "a path with a ./ prefix resolves against the folder of the configuration file",
			value: "./pki",
			want:  filepath.Join(confDir, "pki"),
		},
		{
			desc:  "a path with a ../ prefix resolves against the folder of the configuration file",
			value: "../secrets/pki",
			want:  filepath.Join(string(os.PathSeparator), "home", "user", "secrets", "pki"),
		},
		{
			desc:    "a relative folder of the configuration file gives an absolute path",
			confDir: ".",
			value:   "./pki",
			want:    "pki",
		},
		{
			// The mapper expands the dynamic values before it applies the rules for a relative path, so
			// this check must expand them too.
			desc:  "a path from a {path://} value resolves against the folder of the configuration file",
			value: "{path://pki}",
			want:  filepath.Join(confDir, "pki"),
		},
		{
			desc:  "a path from a {path://} value with a subfolder",
			value: "{path://secrets/pki}",
			want:  filepath.Join(confDir, "secrets", "pki"),
		},
		{
			desc:    "a relative path without a ./ prefix is an error",
			value:   "pki",
			wantErr: clusterpki.ErrPathNotResolvable,
		},
		{
			desc:    "a relative path with a folder and without a ./ prefix is an error",
			value:   "secrets/pki",
			wantErr: clusterpki.ErrPathNotResolvable,
		},
		{
			desc:    "an empty value is an error",
			value:   "",
			wantErr: clusterpki.ErrPathEmpty,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir := confDir
			if tC.confDir != "" {
				dir = tC.confDir
			}

			got, err := clusterpki.ResolvePath(tC.value, dir)

			if tC.wantErr != nil {
				require.True(t, errors.Is(err, tC.wantErr), "got error %v", err)

				return
			}

			require.NoError(t, err)

			// The result is always absolute, so compare against the absolute form of the wanted path.
			want, absErr := filepath.Abs(tC.want)
			require.NoError(t, absErr)

			assert.Equal(t, want, got)
			assert.True(t, filepath.IsAbs(got), "the result must be absolute, got %s", got)
		})
	}
}

// TestResolvePath_EnvValue covers a value that comes from an environment variable. The mapper expands
// it before it renders the playbooks, so this check must accept it.
func TestResolvePath_EnvValue(t *testing.T) {
	pkiDir := filepath.Join(string(os.PathSeparator), "opt", "pki")
	t.Setenv("FURYCTL_TEST_PKI_DIR", pkiDir)

	got, err := clusterpki.ResolvePath("{env://FURYCTL_TEST_PKI_DIR}", filepath.Join("home", "user"))

	require.NoError(t, err)
	assert.Equal(t, pkiDir, got)
}

// TestResolvePath_EnvValueEmpty covers an environment variable that holds no value. The parser reports
// it, so the message names the variable.
func TestResolvePath_EnvValueEmpty(t *testing.T) {
	t.Setenv("FURYCTL_TEST_PKI_DIR", "")

	_, err := clusterpki.ResolvePath("{env://FURYCTL_TEST_PKI_DIR}", filepath.Join("home", "user"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FURYCTL_TEST_PKI_DIR")
}
