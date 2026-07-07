// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"

	gogetterx "github.com/sighupio/furyctl/internal/x/go-getter"
)

// headProbeTimeout bounds the HEAD size probe so a slow server can't hold up the download.
const headProbeTimeout = 10 * time.Second

var ErrDownloadOptionsExhausted = errors.New("downloading options exhausted")

func NewGoGetterClient() *GoGetterClient {
	return &GoGetterClient{
		protocols: []string{"", "git::", "file::", "http::", "s3::", "gcs::", "mercurial::"},
	}
}

type GoGetterClient struct {
	protocols []string
}

func (*GoGetterClient) Clear() error {
	return nil
}

func (*GoGetterClient) ClearItem(_ string) error {
	return nil
}

func (g *GoGetterClient) Download(src, dst string) error {
	return g.DownloadWithMode(src, dst, getter.ClientModeAny, nil)
}

// DownloadWithMode allows downloading with a specific mode (File, Dir, or Any).
// The decompressors map can be used to specify custom decompressors or disable built-in ones by passing an empty map.
func (g *GoGetterClient) DownloadWithMode(
	src, dst string,
	mode getter.ClientMode,
	decompressors map[string]getter.Decompressor,
) error {
	protocols := []string{""}
	if !g.URLHasForcedProtocol(src) {
		protocols = g.protocols
	}

	for _, protocol := range protocols {
		fullSrc := fmt.Sprintf("%s%s", protocol, src)

		logrus.Debugf("Downloading '%s' in '%s'", fullSrc, dst)
		client := &getter.Client{
			Src:  fullSrc,
			Dst:  dst,
			Mode: mode,
			Getters: map[string]getter.Getter{
				"file": &gogetterx.FileGetter{
					Copy: true,
				},
				"git": new(getter.GitGetter),
				"gcs": new(getter.GCSGetter),
				"hg":  new(getter.HgGetter),
				"s3":  new(getter.S3Getter),
				"http": &getter.HttpGetter{
					Netrc: true,
				},
				"https": &getter.HttpGetter{
					Netrc: true,
				},
			},
			DisableSymlinks: false,
		}

		// Report progress on large downloads. The tracker self-gates on size, and go-getter only
		// invokes it for the http/s3 getters (git/file/gcs/hg clones are silent). The HEAD probe
		// supplies a size when the GET response has no Content-Length.
		tracker := newProgressTracker()
		tracker.fallbackTotal = probeContentLength(fullSrc)
		client.ProgressListener = tracker

		// When downloading a single file we don't want go-getter to auto-decompress
		// archives (eg. .bz2). An empty map disables the built-in decompressors.
		if decompressors != nil {
			client.Decompressors = decompressors
		}

		err := client.Get()
		if err == nil {
			return nil
		}

		logrus.Debug(err)
	}

	return ErrDownloadOptionsExhausted
}

// probeContentLength HEADs a download to learn its size, for when the GET response has no
// Content-Length (the Flatcar CDN streams over HTTP/2 without one, though a HEAD reports the size).
// Returns -1 if unknown. Only http(s) URLs are probed; other getters don't report progress.
func probeContentLength(src string) int64 {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return -1
	}

	ctx, cancel := context.WithTimeout(context.Background(), headProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, src, nil)
	if err != nil {
		return -1
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Debugf("progress: HEAD probe for '%s' failed: %v", src, err)

		return -1
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return -1
	}

	return resp.ContentLength
}

// URLHasForcedProtocol checks if the url has a forced protocol as described in hashicorp/go-getter.
func (g *GoGetterClient) URLHasForcedProtocol(url string) bool {
	for _, dp := range g.protocols {
		if dp != "" && strings.HasPrefix(url, dp) {
			return true
		}
	}

	return false
}
