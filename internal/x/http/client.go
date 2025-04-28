// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"errors"
	"net/http"
	"slices"
)

const MaxAllowedRedirects = 10

var (
	//nolint:gochecknoglobals // following in the steps of the standard library
	DefaultClient = NewClient([]string{"github.com", "gitlab.com", "sighup.io"})

	ErrNoRequest                   = errors.New("no request")
	ErrMaxAllowedRedirectsExceeded = errors.New("maximum number of allowed redirects exceeded")
)

// NewClient instantiate a safer HTTP Client that only allows certain redirects.
func NewClient(allowedDomains []string) *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req == nil {
				return ErrNoRequest
			}

			if len(via) >= MaxAllowedRedirects {
				return ErrMaxAllowedRedirectsExceeded
			}

			if slices.Contains(allowedDomains, req.Host) {
				return nil
			}

			return http.ErrUseLastResponse
		},
	}
}
