// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

const (
	progressTTYInterval = 500 * time.Millisecond // Repaint cadence for the animated line.
	progressLogInterval = 5 * time.Second        // Cadence for the fallback log lines.

	// Downloads smaller than this aren't worth reporting progress for.
	progressMinTrackedSize = 5 * 1000 * 1000

	bytesUnit    = 1000
	percentScale = 100
)

// downloadProgressTracker implements getter.ProgressTracker so large downloads (Flatcar boot images
// are hundreds of MB) aren't mistaken for a hung process.
type downloadProgressTracker struct {
	out io.Writer
	// Whether progress can be drawn as an in-place line; false on non-terminals, --no-tty and --debug.
	tty bool
	// Size from a HEAD probe, used when the GET response has no Content-Length.
	fallbackTotal int64
}

// newProgressTracker builds the tracker for a download. It is a package var, not a plain func, so
// tests can swap in one that writes to a buffer.
//
//nolint:gochecknoglobals // Overridable constructor so tests can inject a buffer-backed tracker.
var newProgressTracker = func() *downloadProgressTracker {
	f := os.Stderr

	return &downloadProgressTracker{
		// Animate only on a terminal; under --no-tty/--debug fall back to logs, as the dependency
		// downloader does for its live region.
		out: f,
		tty: execx.ShouldAnimate(f),
	}
}

func (t *downloadProgressTracker) interval() time.Duration {
	if t.tty {
		return progressTTYInterval
	}

	return progressLogInterval
}

// TrackProgress wraps the download stream in a reader that reports progress as it is read.
func (t *downloadProgressTracker) TrackProgress(
	src string,
	currentSize, totalSize int64,
	stream io.ReadCloser,
) io.ReadCloser {
	// A chunked/HTTP2 GET reports no Content-Length (totalSize <= 0); use the HEAD-probed size instead.
	if totalSize <= 0 && t.fallbackTotal > 0 {
		totalSize = t.fallbackTotal
	}

	return &progressReader{
		tracker: t,
		stream:  stream,
		name:    filepath.Base(src),
		read:    currentSize,
		total:   totalSize,
	}
}

// progressReader counts bytes read from the download stream and renders progress, throttled by the
// tracker's interval.
type progressReader struct {
	tracker    *downloadProgressTracker
	stream     io.ReadCloser
	name       string
	read       int64
	total      int64
	lastRender time.Time
	started    bool
}

// tracked reports whether the download is large enough to report. With a known size we decide upfront;
// with an unknown size we start once enough bytes have streamed to prove it's large.
func (r *progressReader) tracked() bool {
	if r.total >= progressMinTrackedSize {
		return true
	}

	return r.total <= 0 && r.read >= progressMinTrackedSize
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.stream.Read(p)
	r.read += int64(n)

	if r.tracked() {
		now := time.Now()
		if !r.started || now.Sub(r.lastRender) >= r.tracker.interval() {
			r.started = true
			r.lastRender = now
			r.render()
		}
	}

	//nolint:wrapcheck // Pass the stream error through unchanged; go-getter handles it.
	return n, err
}

func (r *progressReader) Close() error {
	// Erase the in-place line now the download is done; the caller logs its own completion message.
	if r.started && r.tracker.tty {
		_, _ = fmt.Fprint(r.tracker.out, "\r\033[2K")
	}

	//nolint:wrapcheck // Pass the stream error through unchanged; go-getter handles it.
	return r.stream.Close()
}

func (r *progressReader) render() {
	status := humanizeBytes(r.read)

	if r.total > 0 {
		pct := float64(r.read) / float64(r.total) * percentScale
		status = fmt.Sprintf("%3.0f%% (%s / %s)", pct, status, humanizeBytes(r.total))
	}

	if !r.tracker.tty {
		logrus.Infof("Downloading %s: %s", r.name, status)

		return
	}

	// Rewrite the line in place: \r returns to column 0, then \033[2K clears it.
	_, _ = fmt.Fprintf(r.tracker.out, "\r\033[2K  Downloading %s: %s", r.name, status)
}

// humanizeBytes formats a byte count using metric (base-1000) units.
func humanizeBytes(b int64) string {
	if b < bytesUnit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(bytesUnit), 0
	for n := b / bytesUnit; n >= bytesUnit; n /= bytesUnit {
		div *= bytesUnit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
