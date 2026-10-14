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

func TestDoDownloadSkipsTopDotFilesOfLocalDistribution(t *testing.T) {
	kfd, err := os.ReadFile("../../internal/state/test_data/kfd.yaml")
	require.NoError(t, err)

	distro := t.TempDir()

	for name, content := range map[string][]byte{
		"kfd.yaml":                              kfd,
		"templates/distribution/a.yaml.tpl":     []byte("a"),
		"templates/distribution/.gitignore.tpl": []byte("**"),
		".git/HEAD":                             []byte("ref: refs/heads/main"),
		".claude/worktrees/w/kfd.yaml":          kfd,
	} {
		path := filepath.Join(distro, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, content, 0o600))
	}

	t.Setenv("TMPDIR", t.TempDir())
	t.Cleanup(osx.RemoveTempDirs)

	client := netx.NewGoGetterClient()

	res, err := dist.NewDownloader(client, git.ProtocolHTTPS, "").DoDownload(distro, config.Furyctl{})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(res.RepoPath, "kfd.yaml"))
	require.FileExists(t, filepath.Join(res.RepoPath, "templates", "distribution", "a.yaml.tpl"))
	require.FileExists(t, filepath.Join(res.RepoPath, "templates", "distribution", ".gitignore.tpl"),
		"furyctl copies a dot file below the top")
	require.NoDirExists(t, filepath.Join(res.RepoPath, ".git"))
	require.NoDirExists(t, filepath.Join(res.RepoPath, ".claude"))

	// The commands share the client with the other downloads, for example the dependencies. The
	// client must still copy the dot files.
	dst := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, client.Download(distro, dst))
	require.DirExists(t, filepath.Join(dst, ".git"))
}
