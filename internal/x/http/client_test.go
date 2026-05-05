// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http_test

import (
	"net/http"
	"testing"

	"github.com/sighupio/furyctl/internal/test"
	httpx "github.com/sighupio/furyctl/internal/x/http"
)

func TestClient_CheckRedirect(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		req     *http.Request
		via     []*http.Request
		wantErr error
	}{
		{
			desc:    "no request",
			req:     nil,
			via:     nil,
			wantErr: httpx.ErrNoRequest,
		},
		{
			desc: "too many redirects",
			req:  req(t, "https://sighup.io"),
			via: []*http.Request{
				req(t, "https://sighup.io/0"),
				req(t, "https://sighup.io/1"),
				req(t, "https://sighup.io/2"),
				req(t, "https://sighup.io/3"),
				req(t, "https://sighup.io/4"),
				req(t, "https://sighup.io/5"),
				req(t, "https://sighup.io/6"),
				req(t, "https://sighup.io/7"),
				req(t, "https://sighup.io/8"),
				req(t, "https://sighup.io/9"),
				req(t, "https://sighup.io/10"),
			},
			wantErr: httpx.ErrMaxAllowedRedirectsExceeded,
		},
		{
			desc:    "allow https redirect",
			req:     req(t, "https://sighup.io"),
			via:     nil,
			wantErr: nil,
		},
		{
			desc:    "allow http redirect",
			req:     req(t, "http://sighup.io"),
			via:     nil,
			wantErr: nil,
		},
		{
			desc:    "disallow redirect",
			req:     req(t, "https://example.dev"),
			via:     nil,
			wantErr: http.ErrUseLastResponse,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			client := httpx.NewClient([]string{"sighup.io"})

			err := client.CheckRedirect(tC.req, tC.via)

			test.AssertErrorIs(t, err, tC.wantErr)
		})
	}
}

func req(t *testing.T, url string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}

	return req
}
