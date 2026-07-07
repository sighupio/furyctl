// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx //nolint:testpackage // Exercises unexported progress internals (tracker seam, humanizeBytes, probe).

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/go-getter"
)

func TestHumanizeBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1500, "1.5 kB"},
		{265_000_000, "265.0 MB"},
		{3_200_000_000, "3.2 GB"},
	}

	for _, c := range cases {
		if got := humanizeBytes(c.in); got != c.want {
			t.Errorf("humanizeBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

const (
	largePayloadSize = 8_000_000 // Above progressMinTrackedSize.
	smallPayloadSize = 1_000_000 // Below progressMinTrackedSize.
)

// writeChunked streams the body without a Content-Length, so the GET reports an unknown total.
func writeChunked(w http.ResponseWriter, payload []byte) {
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)

	const chunk = 1_000_000
	for off := 0; off < len(payload); off += chunk {
		end := off + chunk
		if end > len(payload) {
			end = len(payload)
		}

		_, _ = w.Write(payload[off:end])

		if ok {
			flusher.Flush()
		}
	}
}

// captureDownload downloads url and returns what the tracker rendered (forced to terminal mode).
func captureDownload(t *testing.T, url string) string {
	t.Helper()

	var buf bytes.Buffer

	orig := newProgressTracker
	newProgressTracker = func() *downloadProgressTracker {
		return &downloadProgressTracker{out: &buf, tty: true}
	}

	t.Cleanup(func() { newProgressTracker = orig })

	dst := filepath.Join(t.TempDir(), "asset.bin")
	if err := NewGoGetterClient().DownloadWithMode(
		url, dst, getter.ClientModeFile, map[string]getter.Decompressor{},
	); err != nil {
		t.Fatalf("download: %v", err)
	}

	return buf.String()
}

//nolint:paralleltest // Mutates the global newProgressTracker seam.
func TestDownloadProgressKnownContentLength(t *testing.T) {
	payload := bytes.Repeat([]byte("A"), largePayloadSize)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	out := captureDownload(t, srv.URL+"/asset.bin")

	if !strings.Contains(out, "%") || !strings.Contains(out, "8.0 MB") {
		t.Fatalf("expected a percentage against an 8.0 MB total; got %q", clean(out))
	}

	// The line is erased on completion, so the output ends with the clear sequence, not a stale bar.
	if !strings.HasSuffix(out, "\r\033[2K") {
		t.Fatalf("expected the progress line to be cleared on completion; got %q", clean(out))
	}
}

//nolint:paralleltest // Mutates the global newProgressTracker seam.
func TestDownloadProgressHeadFallbackWhenGetHasNoContentLength(t *testing.T) {
	payload := bytes.Repeat([]byte("A"), largePayloadSize)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HEAD advertises the size; GET streams it with no Content-Length (the Flatcar CDN case).
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)

			return
		}

		writeChunked(w, payload)
	}))
	defer srv.Close()

	out := captureDownload(t, srv.URL+"/flatcar.vmlinuz")

	if !strings.Contains(out, "%") || !strings.Contains(out, "8.0 MB") {
		t.Fatalf("expected the HEAD probe to supply an 8.0 MB total and a percentage; got %q", clean(out))
	}
}

//nolint:paralleltest // Mutates the global newProgressTracker seam.
func TestDownloadProgressUnknownTotalShowsBytesOnly(t *testing.T) {
	payload := bytes.Repeat([]byte("A"), largePayloadSize)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Neither HEAD nor GET expose a size, so we can only report bytes downloaded.
		writeChunked(w, payload)
	}))
	defer srv.Close()

	out := captureDownload(t, srv.URL+"/asset.bin")

	if !strings.Contains(out, "Downloading") {
		t.Fatalf("expected progress to be reported; got %q", clean(out))
	}

	if strings.Contains(out, "%") {
		t.Fatalf("expected bytes-only progress (no percentage) when the total is unknown; got %q", clean(out))
	}
}

//nolint:paralleltest // Mutates the global newProgressTracker seam.
func TestDownloadProgressSkipsSmallFiles(t *testing.T) {
	payload := bytes.Repeat([]byte("A"), smallPayloadSize)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	if out := captureDownload(t, srv.URL+"/small.bin"); out != "" {
		t.Fatalf("expected no progress for a sub-threshold file; got %q", clean(out))
	}
}

func TestProbeContentLength(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "12345")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if got := probeContentLength(srv.URL + "/x"); got != 12345 {
		t.Errorf("probeContentLength(http) = %d, want 12345", got)
	}

	// Non-http(s) sources are not probed.
	if got := probeContentLength("git::https://example.com/repo"); got != -1 {
		t.Errorf("probeContentLength(git) = %d, want -1", got)
	}
}

// clean makes the in-place terminal escape sequences readable in test failure output.
func clean(s string) string {
	return strings.ReplaceAll(s, "\r\033[2K", " | ")
}
