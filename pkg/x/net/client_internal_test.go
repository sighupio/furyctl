// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package netx //nolint:testpackage // exercises the unexported cache-validity test.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An interrupted copy to the cache must leave no entry: a later run uses any entry that exists.
func TestCopyDownloadToLocalCache(t *testing.T) {
	t.Parallel()

	src := "https://github.com/sighupio/fury-distribution?ref=v1.35.1"

	testCases := map[string]struct {
		cancelled   bool
		entryExists bool
		wantErr     error
		wantEntries int
	}{
		"complete copy":  {wantEntries: 1},
		"cancelled copy": {cancelled: true, wantErr: context.Canceled},
		// A parallel run created the entry during the copy.
		"entry exists": {entryExists: true, wantEntries: 1},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			download := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(download, "kfd.yaml"), []byte("x"), 0o600))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.cancelled {
				cancel()
			}

			d := &LocalCacheClientDecorator{dir: t.TempDir()}

			if tc.entryExists {
				require.NoError(t, os.MkdirAll(d.getKeyFromURL(src), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(d.getKeyFromURL(src), "kfd.yaml"), []byte("x"), 0o600))
			}

			err := d.copyDownloadToLocalCache(ctx, src, download)
			require.ErrorIs(t, err, tc.wantErr)

			entries, err := os.ReadDir(d.dir)
			require.NoError(t, err)
			assert.Len(t, entries, tc.wantEntries)

			if tc.wantEntries > 0 {
				assert.FileExists(t, filepath.Join(d.getKeyFromURL(src), "kfd.yaml"))
			}
		})
	}
}

// A killed run leaves a partial folder. Only an old one goes, because a parallel run can use a new one.
func TestRemoveStalePartials(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stale := filepath.Join(dir, "partial-stale")
	fresh := filepath.Join(dir, "partial-fresh")

	require.NoError(t, os.Mkdir(stale, 0o755))
	require.NoError(t, os.Mkdir(fresh, 0o755))

	old := time.Now().Add(-2 * stalePartialAge)
	require.NoError(t, os.Chtimes(stale, old, old))

	removeStalePartials(dir)

	assert.NoDirExists(t, stale)
	assert.DirExists(t, fresh)
}

// The cache key is the URL, so only a ref that always resolves to the same commit
// can be served from the cache. A branch must download again every time.
func TestImmutableRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			"version tag",
			"https://github.com/sighupio/fury-distribution?ref=v1.35.1&depth=1",
			true,
		},
		{
			"version tag without the v prefix",
			"https://github.com/sighupio/installer-immutable?ref=1.35.1&depth=1",
			true,
		},
		{
			"commit sha",
			"https://github.com/sighupio/installer-immutable?ref=2870af80d9fea1dc426a65676ec79d02756fcbe7&depth=1",
			true,
		},
		{
			// The ref that made a pushed fix invisible to every later run.
			"feature branch",
			"https://github.com/sighupio/installer-immutable?ref=feat/kubernetes-1.36.4-1.35.8&depth=1",
			false,
		},
		{
			"main branch",
			"https://github.com/sighupio/installer-immutable?ref=main&depth=1",
			false,
		},
		{
			"branch that starts with a version",
			"https://github.com/sighupio/installer-immutable?ref=1.36.4-fix/lb&depth=1",
			false,
		},
		{
			"no ref at all",
			"https://github.com/sighupio/installer-immutable",
			false,
		},
		{
			"empty ref",
			"https://github.com/sighupio/installer-immutable?ref=&depth=1",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, immutableRef(test.src))
		})
	}
}
