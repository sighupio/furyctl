// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package serve

import (
	"net"
	"net/http"
	"testing"
	"time"
)

// A machine that the user resets in the middle of a transfer leaves a request that never ends.
// The shutdown of the server reaches its timeout, and the apply must continue, because every
// machine booted already and the assets are no longer needed.
//
// The test holds a request open, as a reset machine does, and it reads two results: that
// stopServer gives control back, and that it ends the request that remains.
func TestStopServerWithARequestThatNeverEnds(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	started := make(chan struct{})

	defer close(release)

	srv := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			close(started)
			<-release
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("error listening: %v", err)
	}

	go func() { _ = srv.Serve(listener) }()

	clientDone := make(chan error, 1)

	go func() {
		// The request outlives the shutdown on purpose, so it takes no context.
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			clientDone <- err

			return
		}

		// The handler holds the body open until the server ends the connection.
		_, err = resp.Body.Read(make([]byte, 1))
		resp.Body.Close()
		clientDone <- err
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not start")
	}

	returned := make(chan struct{})

	go func() {
		stopServer(srv, 100*time.Millisecond)
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("stopServer did not give control back after the timeout")
	}

	// Shutdown reached its timeout, so the connection that remains needs a Close. Without
	// one the client waits for a handler that never returns.
	select {
	case <-clientDone:
	case <-time.After(3 * time.Second):
		t.Error("stopServer left the open request alive")
	}
}
