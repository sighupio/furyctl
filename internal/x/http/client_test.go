// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

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
			req:  req("https://sighup.io"),
			via: []*http.Request{
				req("https://sighup.io/0"),
				req("https://sighup.io/1"),
				req("https://sighup.io/2"),
				req("https://sighup.io/3"),
				req("https://sighup.io/4"),
				req("https://sighup.io/5"),
				req("https://sighup.io/6"),
				req("https://sighup.io/7"),
				req("https://sighup.io/8"),
				req("https://sighup.io/9"),
				req("https://sighup.io/10"),
			},
			wantErr: httpx.ErrMaxAllowedRedirectsExceeded,
		},
		{
			desc:    "allow https redirect",
			req:     req("https://sighup.io"),
			via:     nil,
			wantErr: nil,
		},
		{
			desc:    "allow http redirect",
			req:     req("http://sighup.io"),
			via:     nil,
			wantErr: nil,
		},
		{
			desc:    "disallow redirect",
			req:     req("https://example.dev"),
			via:     nil,
			wantErr: http.ErrUseLastResponse,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			client := httpx.NewClient([]string{"sighup.io"})

			err := client.CheckRedirect(tC.req, tC.via)

			assertErrorIs(t, err, tC.wantErr)
		})
	}
}

func req(url string) *http.Request {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}

	return req
}

func assertErrorIs(t *testing.T, err, want error) {
	t.Helper()

	if want == nil {
		require.NoError(t, err)
	} else {
		require.ErrorIs(t, err, want)
	}
}
