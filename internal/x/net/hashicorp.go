// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"

	gogetterx "github.com/sighupio/furyctl/internal/x/go-getter"
)

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

func (g *GoGetterClient) Download(src, dst string) error {
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
			Mode: getter.ClientModeAny,
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

		err := client.Get()
		if err == nil {
			return nil
		}

		logrus.Debug(err)
	}

	return ErrDownloadOptionsExhausted
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
