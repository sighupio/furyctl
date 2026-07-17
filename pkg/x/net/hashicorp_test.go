// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package netx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	netx "github.com/sighupio/furyctl/pkg/x/net"
)

func Test_GoGetterClient_Download(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "furyctl-clientget-test-")
	require.NoError(t, err, "error creating temp dir")

	in := filepath.Join(tmpDir, "in")
	out := filepath.Join(tmpDir, "out")

	err = os.MkdirAll(in, 0o755)
	require.NoError(t, err, "error creating temp dir")

	src, err := os.Create(filepath.Join(in, "test.txt"))
	require.NoError(t, err, "error creating temp input file")

	defer func() {
		src.Close()

		_ = os.RemoveAll(tmpDir)
	}()

	err = netx.NewGoGetterClient().Download(in, out)
	require.NoError(t, err, "error getting directory")

	_, err = os.Stat(filepath.Join(out, "test.txt"))
	require.NoError(t, err, "error getting file")
}

func TestUrlHasForcedProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "test with http",
			url:  "http::test.com",
			want: true,
		},
		{
			name: "test without protocol",
			url:  "test.com",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := netx.NewGoGetterClient().URLHasForcedProtocol(tt.url)
			assert.Equal(t, tt.want, got, "urlHasForcedProtocol()")
		})
	}
}
