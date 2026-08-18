// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable"
	"github.com/sighupio/furyctl/internal/clusterpki"
)

// writeConf writes a configuration file that holds the given pkiPath value, and returns its path. An
// empty value writes no field at all.
func writeConf(t *testing.T, dir, value string) string {
	t.Helper()

	conf := "apiVersion: kfd.sighup.io/v1alpha2\nkind: Immutable\nspec:\n  kubernetes:\n"
	if value != "" {
		conf += "    pkiPath: " + value + "\n"
	}

	path := filepath.Join(dir, "furyctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(conf), 0o600))

	return path
}

// writePKI writes a complete PKI folder at dir/pki.
func writePKI(t *testing.T, dir string) {
	t.Helper()

	files := map[string][]string{
		"master": {"ca.crt", "ca.key", "front-proxy-ca.crt", "front-proxy-ca.key", "sa.key", "sa.pub"},
		"etcd":   {"ca.crt", "ca.key"},
	}

	for sub, names := range files {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "pki", sub), os.ModePerm))

		for _, name := range names {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "pki", sub, name), []byte("x"), 0o600))
		}
	}
}

func TestPKIValidator_ValidatePKI_Complete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	confPath := writeConf(t, dir, "./pki")
	writePKI(t, dir)

	validator := &immutable.PKIValidator{}

	require.NoError(t, validator.ValidatePKI(confPath))
}

func TestPKIValidator_ValidatePKI_FolderMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	confPath := writeConf(t, dir, "./pki")

	validator := &immutable.PKIValidator{}

	err := validator.ValidatePKI(confPath)

	require.Error(t, err)
	assert.ErrorIs(t, err, immutable.ErrPkiPath)

	folderMissing := &clusterpki.FolderMissingError{}
	require.ErrorAs(t, err, &folderMissing)
	assert.Equal(t, filepath.Join(dir, "pki"), folderMissing.Path)
}

func TestPKIValidator_ValidatePKI_ValueErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		value   string
		wantErr error
	}{
		{
			desc:    "the value is relative without a ./ prefix",
			value:   "pki",
			wantErr: clusterpki.ErrPathNotResolvable,
		},
		{
			desc:    "the field is absent",
			value:   "",
			wantErr: clusterpki.ErrPathEmpty,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			confPath := writeConf(t, dir, tC.value)
			writePKI(t, dir)

			validator := &immutable.PKIValidator{}

			err := validator.ValidatePKI(confPath)

			require.Error(t, err)
			assert.ErrorIs(t, err, immutable.ErrPkiPath)
			assert.ErrorIs(t, err, tC.wantErr)
		})
	}
}
