// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package serve

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// loggingResponseWriter wraps http.ResponseWriter to capture status code and bytes written.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader captures the status code and delegates to the underlying ResponseWriter.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written and ensures a default status code.
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.status == 0 {
		lrw.status = http.StatusOK
	}

	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytes += n

	if err != nil {
		return n, fmt.Errorf("error writing response: %w", err)
	}

	return n, nil
}

// Start an HTTP server serving a path in the file system on a custom address and port, logging each request.
// The server is stopped when the user presses ENTER.
// Note: Path traversal attacks, serving directory listings, dot files, and other security vulnerabilities should be addressed.
func Path(address, port, root string) error {
	fs := http.FileServer(http.Dir(root))

	// Wrap the file server with a logging handler that logs each request.
	typedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use package-level loggingResponseWriter.
		lrw := &loggingResponseWriter{ResponseWriter: w}
		fs.ServeHTTP(lrw, r)

		// Log relevant request/response information.
		logrus.WithFields(logrus.Fields{
			"remote":     r.RemoteAddr,
			"user-agent": r.Header.Get("User-Agent"),
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     lrw.status,
			"bytes":      lrw.bytes,
		}).Info("served request")
	})

	http.Handle("/", typedHandler)

	listenAddr := strings.Join([]string{address, port}, ":")
	logrus.WithFields(logrus.Fields{
		"address": address,
		"port":    port,
		"root":    root,
	}).Warn("Serving assets, you can boot your machines now. Press ENTER to stop the server and continue or CTRL+C to cancel...")

	const readHeaderTimeout = 5 * time.Second

	// Create server so we can control shutdown and inspect errors.
	srv := &http.Server{Addr: listenAddr, Handler: nil, ReadHeaderTimeout: readHeaderTimeout}

	// Channel to receive server errors.
	errCh := make(chan error, 1)
	go func() {
		// ListenAndServe returns http.ErrServerClosed on graceful shutdown.
		err := srv.ListenAndServe()
		errCh <- err
	}()

	// Channel to signal user requested stop (ENTER).
	inputCh := make(chan struct{})
	go func() {
		_, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
		if err != nil {
			errCh <- err
		}

		close(inputCh)
	}()

	// Wait for either user input or server error.
	select {
	case <-inputCh:
		logrus.Info("Shutdown requested by user, shutting down HTTP server...")

		const shutdownTimeout = 5 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)

		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("error during server shutdown: %w", err)
		}

		logrus.Info("Server stopped")

		return nil

	case err := <-errCh:
		// If server was closed normally, treat as no error.
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			logrus.Info("HTTP server closed")

			return nil
		}
		// Unexpected error.
		logrus.WithError(err).Error("HTTP server failed")

		return err
	}
}
