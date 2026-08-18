// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package onpremises_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises"
	"github.com/sighupio/furyctl/internal/clusterpki"
)

// writeConf writes a configuration file that holds the given pkiFolder value, and returns its path. An
// empty value writes the field with an empty string, which the older schemas accept.
func writeConf(t *testing.T, dir, value string) string {
	t.Helper()

	conf := "apiVersion: kfd.sighup.io/v1alpha2\nkind: OnPremises\nspec:\n  kubernetes:\n" +
		"    pkiFolder: \"" + value + "\"\n"

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

	validator := &onpremises.PKIValidator{}

	require.NoError(t, validator.ValidatePKI(confPath))
}

func TestPKIValidator_ValidatePKI_FilesMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	confPath := writeConf(t, dir, "./pki")
	writePKI(t, dir)
	require.NoError(t, os.Remove(filepath.Join(dir, "pki", "etcd", "ca.key")))

	validator := &onpremises.PKIValidator{}

	err := validator.ValidatePKI(confPath)

	require.Error(t, err)
	assert.ErrorIs(t, err, onpremises.ErrPkiFolder)

	filesMissing := &clusterpki.FilesMissingError{}
	require.ErrorAs(t, err, &filesMissing)
	assert.Equal(t, []string{filepath.Join("etcd", "ca.key")}, filesMissing.Missing)
}

func TestPKIValidator_ValidatePKI_EmptyValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	confPath := writeConf(t, dir, "")
	writePKI(t, dir)

	validator := &onpremises.PKIValidator{}

	err := validator.ValidatePKI(confPath)

	require.Error(t, err)
	assert.ErrorIs(t, err, onpremises.ErrPkiFolder)
	assert.ErrorIs(t, err, clusterpki.ErrPathEmpty)
}
