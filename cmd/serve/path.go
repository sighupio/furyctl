// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package serve

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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
// The server stops when the user presses ENTER or once every node reports "booted".
func Path(address, port, root string, nodesStatus map[string]string) error {
	// ENTER and all-nodes-booted both cancel this context to stop the server; cancel() is
	// idempotent, so the two paths can't race into a double-stop panic.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Live view of node bootstrap status; owns its own locking for concurrent status POSTs.
	table := newNodeStatusTable(nodesStatus)

	var bootedOnce sync.Once

	// Own mux (not http.DefaultServeMux) so repeated Path calls can't panic on re-registration.
	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir(root))

	// Wrap the file server with a logging handler that logs each request.
	typedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalize MAC addresses to uppercase in /boot/ paths because iPXE sends lowercase MACs (bc-24-11-cc-dd-01)
		// in the URL, but files are uppercase (BC-24-11-CC-DD-01).
		if strings.HasPrefix(r.URL.Path, "/boot/") {
			macPart := strings.TrimPrefix(r.URL.Path, "/boot/")
			r.URL.Path = "/boot/" + strings.ToUpper(macPart)
		}

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
		}).Debug("served asset request")
	})

	// Serves the /status endpoint: GET returns the node status map, POST records an update.
	statusHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use package-level loggingResponseWriter.
		lrw := &loggingResponseWriter{ResponseWriter: w}

		if r.Method == http.MethodGet {
			lrw.Header().Set("Content-Type", "application/json")
			lrw.WriteHeader(http.StatusOK)
			encoder := json.NewEncoder(lrw)

			err := encoder.Encode(table.Snapshot())
			if err != nil {
				logrus.Errorf("error while encoding response: %s", err)
			} else {
				logrus.WithFields(logrus.Fields{
					"remote":     r.RemoteAddr,
					"user-agent": r.Header.Get("User-Agent"),
					"method":     r.Method,
					"path":       r.URL.Path,
					"status":     lrw.status,
					"bytes":      lrw.bytes,
				}).Debug("served nodes status")
			}
		}

		if r.Method == http.MethodPost {
			lrw.WriteHeader(http.StatusNoContent)

			// Update node status based on query parameters.
			node := r.URL.Query().Get("node")
			status := r.URL.Query().Get("status")

			if node == "" || status == "" {
				logrus.WithFields(logrus.Fields{
					"remote":     r.RemoteAddr,
					"user-agent": r.Header.Get("User-Agent"),
					"method":     r.Method,
					"path":       r.URL.Path,
				}).Warn("received status update with missing node or status query parameters")

				return
			}

			// Debug log for the machine-readable history; the human-facing view is the table.
			logrus.WithFields(logrus.Fields{
				"remote":     r.RemoteAddr,
				"user-agent": r.Header.Get("User-Agent"),
				"method":     r.Method,
				"path":       r.URL.Path,
				"respStatus": lrw.status,
				"respBytes":  lrw.bytes,
				"hostname":   node,
				"nodeStatus": status,
			}).Debug("received node status update")

			table.Update(node, status)

			if table.AllBooted() {
				bootedOnce.Do(func() {
					logrus.Infof("All %d nodes reached 'booted' state. Stopping server and continuing...", table.Len())
					cancel()
				})
			}
		}
	})

	mux.Handle("/status", statusHandler)
	mux.Handle("/", typedHandler)

	listenAddr := strings.Join([]string{address, port}, ":")
	logrus.WithFields(logrus.Fields{
		"address": address,
		"port":    port,
		"root":    root,
	}).Warn("Assets server started. You can boot your machines now")
	logrus.Info("Note: you can press ENTER to skip waiting for all nodes to boot and continue or CTRL+C to cancel and exit at any time")

	// Draw the initial table so every node shows up (as "pending") the moment the server is ready.
	table.Start()

	const readHeaderTimeout = 5 * time.Second

	// Create server so we can control shutdown and inspect errors.
	srv := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: readHeaderTimeout}

	// Channel to receive server errors.
	errCh := make(chan error, 1)
	go func() {
		// ListenAndServe returns http.ErrServerClosed on graceful shutdown.
		errCh <- srv.ListenAndServe()
	}()

	// Stop the server when the operator presses ENTER. A read error (e.g. EOF on a
	// non-interactive stdin) is not fatal: keep serving until all nodes have booted.
	go func() {
		if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
			logrus.Debugf("stopped watching stdin for the stop signal: %v", err)

			return
		}

		cancel()
	}()

	// Wait for either a stop request (ENTER / all nodes booted) or a server error.
	select {
	case <-ctx.Done():
		const shutdownTimeout = 5 * time.Second

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)

		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
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
