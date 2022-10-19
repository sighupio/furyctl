// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package netx_test

import (
	"os"
	"path/filepath"
	"testing"

	netx "github.com/sighupio/furyctl/internal/x/net"
)

func Test_GoGetterClient_Download(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "furyctl-clientget-test-")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	in := filepath.Join(tmpDir, "in")
	out := filepath.Join(tmpDir, "out")

	if err := os.MkdirAll(in, 0o755); err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	src, err := os.Create(filepath.Join(in, "test.txt"))
	if err != nil {
		t.Fatalf("error creating temp input file: %v", err)
	}

	defer func() {
		src.Close()
		_ = os.RemoveAll(tmpDir)
	}()

	err = netx.NewGoGetterClient().Download(in, out)
	if err != nil {
		t.Fatalf("error getting directory: %v", err)
	}

	_, err = os.Stat(filepath.Join(out, "test.txt"))
	if err != nil {
		t.Fatalf("error getting file: %v", err)
	}
}

func TestUrlHasForcedProtocol(t *testing.T) {
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
			if got := netx.NewGoGetterClient().UrlHasForcedProtocol(tt.url); got != tt.want {
				t.Errorf("urlHasForcedProtocol() = %v, want %v", got, tt.want)
			}
		})
	}
}
