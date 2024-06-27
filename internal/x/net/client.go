// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/sighupio/furyctl/internal/git"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	ErrCannotCacheDownload          = errors.New("cannot cache download")
	ErrCannotCheckLocalCache        = errors.New("cannot check local cache")
	ErrCannotGetKeyFromURL          = errors.New("cannot get key from url")
	ErrCannotCopyCacheToDestination = errors.New("cannot copy cache to destination")
	ErrCannotClearCache             = errors.New("cannot clear cache")
	URLPrefixRegexp                 = regexp.MustCompile(`^[A-z0-9]+::`)
)

type Client interface {
	Download(src, dst string) error
	Clear() error
	ClearItem(src string) error
}

func WithLocalCache(c Client, dir string) Client {
	return &LocalCacheClientDecorator{
		client: c,
		dir:    dir,
	}
}

type LocalCacheClientDecorator struct {
	client Client
	dir    string
}

func (d *LocalCacheClientDecorator) Clear() error {
	if err := os.RemoveAll(d.dir); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotClearCache, err)
	}

	return nil
}

func (d *LocalCacheClientDecorator) ClearItem(src string) error {
	key := d.getKeyFromURL(src)

	if err := os.RemoveAll(key); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotClearCache, err)
	}

	return nil
}

func (d *LocalCacheClientDecorator) Download(src, dst string) error {
	csrc := URLPrefixRegexp.ReplaceAllString(src, "")

	hlc, err := d.hasLocalCache(csrc)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCannotCacheDownload, err)
	}

	if hlc {
		if _, err := os.Stat(dst); err != nil {
			return d.copyCacheToDestination(csrc, dst)
		}

		return nil
	}

	if err := d.client.Download(src, dst); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotCacheDownload, err)
	}

	if err := d.copyDownloadToLocalCache(csrc, dst); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotCacheDownload, err)
	}

	return nil
}

func (d *LocalCacheClientDecorator) hasLocalCache(src string) (bool, error) {
	key := d.getKeyFromURL(src)

	if _, err := os.Stat(key); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("%w: %w", ErrCannotCheckLocalCache, err)
	}

	return true, nil
}

func (d *LocalCacheClientDecorator) getKeyFromURL(url string) string {
	cleanURL := git.CleanupRepoURL(url)

	urlSum := sha256.Sum256([]byte(cleanURL))

	return filepath.Join(d.dir, hex.EncodeToString(urlSum[:]))
}

func (d *LocalCacheClientDecorator) copyCacheToDestination(cacheFolder, destFolder string) error {
	key := d.getKeyFromURL(cacheFolder)

	if _, err := os.Stat(destFolder); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %w", ErrCannotCopyCacheToDestination, err)
		}

		if err := os.MkdirAll(destFolder, iox.FullPermAccess); err != nil {
			return fmt.Errorf("%w: %w", ErrCannotCopyCacheToDestination, err)
		}
	}

	if err := iox.CopyRecursive(os.DirFS(key), destFolder); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotCopyCacheToDestination, err)
	}

	return nil
}

func (d *LocalCacheClientDecorator) copyDownloadToLocalCache(downloadFolder, cacheFolder string) error {
	key := d.getKeyFromURL(downloadFolder)

	if _, err := os.Stat(key); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %w", ErrCannotCopyCacheToDestination, err)
		}

		if err := os.MkdirAll(key, iox.FullPermAccess); err != nil {
			return fmt.Errorf("%w: %w", ErrCannotCopyCacheToDestination, err)
		}
	}

	if err := iox.CopyRecursive(os.DirFS(cacheFolder), key); err != nil {
		return fmt.Errorf("%w: %w", ErrCannotCopyCacheToDestination, err)
	}

	return nil
}
