// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/git"
	osx "github.com/sighupio/furyctl/internal/x/os"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

func TestDoDownloadRemovesTempDirs(t *testing.T) {
	kfd, err := os.ReadFile("../../internal/state/test_data/kfd.yaml")
	require.NoError(t, err)

	distro := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(distro, "kfd.yaml"), kfd, 0o600))

	emptyDistro := t.TempDir()
	customPatches := t.TempDir()

	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	requireEmpty := func() {
		t.Helper()

		entries, err := os.ReadDir(tmp)
		require.NoError(t, err)
		require.Empty(t, entries)
	}

	d := dist.NewDownloader(netx.NewGoGetterClient(), git.ProtocolHTTPS, customPatches)

	// The custom patches go away at once, the distribution stays until furyctl exits.
	res, err := d.DoDownload(distro, config.Furyctl{})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(res.RepoPath, "kfd.yaml"))

	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	osx.RemoveTempDirs()
	requireEmpty()

	// A download that fails after the copy removes its folder at once.
	_, err = d.DoDownload(emptyDistro, config.Furyctl{})
	require.ErrorIs(t, err, dist.ErrUnsupportedVersion)
	requireEmpty()
}
