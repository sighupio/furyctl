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
)

var (
	downloadProtocols = []string{"", "git::", "file::", "http::", "s3::", "gcs::", "mercurial::"}

	errDownloadOptionsExausted = errors.New("downloading options exausted")
)

func NewGoGetterClient() *GoGetterClient {
	return &GoGetterClient{}
}

type GoGetterClient struct{}

func (g *GoGetterClient) Download(src, dst string) error {
	protocols := []string{""}
	if !UrlHasForcedProtocol(src) {
		protocols = downloadProtocols
	}

	for _, protocol := range protocols {
		fullSrc := fmt.Sprintf("%s%s", protocol, src)

		logrus.Debugf("Trying to download from: %s", fullSrc)

		client := &getter.Client{
			Src:  fullSrc,
			Dst:  dst,
			Mode: getter.ClientModeAny,
		}

		err := client.Get()
		if err == nil {
			return nil
		}

		logrus.Debug(err)
	}

	return errDownloadOptionsExausted
}

// UrlHasForcedProtocol checks if the url has a forced protocol as described in hashicorp/go-getter.
func UrlHasForcedProtocol(url string) bool {
	for _, dp := range downloadProtocols {
		if dp != "" && strings.HasPrefix(url, dp) {
			return true
		}
	}

	return false
}
