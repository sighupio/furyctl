// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/test"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const (
	distroHTTPSURL = "https://github.com/sighupio/fury-distribution.git"
	distroSSHURL   = "git@github.com:sighupio/fury-distribution.git"
)

var (
	errCannotDownload                  = errors.New("cannot download")
	errCannotCreateFakeDistroDstFolder = errors.New("cannot create fake distro dst folder")
)

func NewFakeClient() *FakeClient {
	return &FakeClient{}
}

type FakeClient struct{}

func (f *FakeClient) Clear() error {
	return nil
}

func (f *FakeClient) ClearItem(src string) error {
	return nil
}

func (f *FakeClient) Download(src, dst string) error {
	switch src {
	case distroHTTPSURL, distroSSHURL:
		if err := createFakeDistroDst(dst); err != nil {
			return fmt.Errorf("%w: %w", errCannotDownload, err)
		}
	}

	return nil
}

func TestLocalCacheClientDecorator_Download_ColdCache(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		shasum  string
		src     string
		dst     string
		wantErr error
	}{
		{
			desc:   "cold cache - https",
			shasum: "25ea7ee9d13d1843dfbeff40948be729af77a30503a6681a1d8293c746de527f",
			src:    distroHTTPSURL,
			dst: func() string {
				baseDst, err := os.MkdirTemp("", "furyctl-")
				if err != nil {
					t.Fatal(err)
				}

				return filepath.Join(baseDst, "data")
			}(),
		},
		{
			desc:   "cold cache - ssh",
			shasum: "25ea7ee9d13d1843dfbeff40948be729af77a30503a6681a1d8293c746de527f",
			src:    distroSSHURL,
			dst: func() string {
				baseDst, err := os.MkdirTemp("", "furyctl-")
				if err != nil {
					t.Fatal(err)
				}

				return filepath.Join(baseDst, "data")
			}(),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			cacheDir, err := os.MkdirTemp("", "furyctl-cache-")
			if err != nil {
				t.Fatal(err)
			}

			c := netx.WithLocalCache(NewFakeClient(), cacheDir)

			// Check the files do not exist in cache.
			assert.NoFileExists(t, filepath.Join(cacheDir, tC.shasum, "kfd.yaml"))
			assert.NoFileExists(t, filepath.Join(cacheDir, tC.shasum, "README.md"))

			err = c.Download(tC.src, tC.dst)

			test.AssertErrorIs(t, err, tC.wantErr)

			// Check the files have been downloaded.
			assert.FileExists(t, filepath.Join(tC.dst, "kfd.yaml"))
			assert.FileExists(t, filepath.Join(tC.dst, "README.md"))

			// Check the files have been cached.
			assert.FileExists(t, filepath.Join(cacheDir, tC.shasum, "kfd.yaml"))
			assert.FileExists(t, filepath.Join(cacheDir, tC.shasum, "README.md"))
		})
	}
}

func TestLocalCacheClientDecorator_Download_WarmCache(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		shasum     string
		src        string
		dst        string
		wantErr    error
		cleanupDst bool
	}{
		{
			desc:   "warm cache, no files in dst",
			shasum: "25ea7ee9d13d1843dfbeff40948be729af77a30503a6681a1d8293c746de527f",
			src:    distroHTTPSURL,
			dst: func() string {
				baseDst, err := os.MkdirTemp("", "furyctl-")
				if err != nil {
					t.Fatal(err)
				}

				return filepath.Join(baseDst, "data")
			}(),
		},
		{
			desc:   "warm cache, files already in dst",
			shasum: "25ea7ee9d13d1843dfbeff40948be729af77a30503a6681a1d8293c746de527f",
			src:    distroHTTPSURL,
			dst: func() string {
				baseDst, err := os.MkdirTemp("", "furyctl-")
				if err != nil {
					t.Fatal(err)
				}

				dst := filepath.Join(baseDst, "data")

				if err := createFakeDistroDst(dst); err != nil {
					t.Fatal(err)
				}

				return dst
			}(),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			cacheDir, err := os.MkdirTemp("", "furyctl-cache-")
			if err != nil {
				t.Fatal(err)
			}

			c := netx.WithLocalCache(NewFakeClient(), cacheDir)

			// Warm up the cache.
			if err := createFakeDistroDst(filepath.Join(cacheDir, tC.shasum)); err != nil {
				t.Fatal(err)
			}

			// Exercise the SUT.
			err = c.Download(tC.src, tC.dst)

			test.AssertErrorIs(t, err, tC.wantErr)

			// Check the files have not been downloaded.
			assert.FileExists(t, filepath.Join(tC.dst, "kfd.yaml"))
			assert.FileExists(t, filepath.Join(tC.dst, "README.md"))

			// Check the files have been cached.
			assert.FileExists(t, filepath.Join(cacheDir, tC.shasum, "kfd.yaml"))
			assert.FileExists(t, filepath.Join(cacheDir, tC.shasum, "README.md"))
		})
	}
}

func createFakeDistroDst(dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("%w: %w", errCannotCreateFakeDistroDstFolder, err)
	}

	if err := os.WriteFile(filepath.Join(dst, "kfd.yaml"), []byte("---"), 0o644); err != nil {
		return fmt.Errorf("%w: %w", errCannotCreateFakeDistroDstFolder, err)
	}

	if err := os.WriteFile(filepath.Join(dst, "README.md"), []byte("# Kubernetes Fury Distribution"), 0o644); err != nil {
		return fmt.Errorf("%w: %w", errCannotCreateFakeDistroDstFolder, err)
	}

	return nil
}
