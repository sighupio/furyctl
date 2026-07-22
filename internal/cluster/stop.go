// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"

	"golang.org/x/sync/errgroup"
)

// StopAll runs the given stop functions concurrently, waits for all of them to finish, and returns
// the first non-nil error (or nil if they all succeed).
func StopAll(fns ...func() error) error {
	var eg errgroup.Group

	for _, fn := range fns {
		eg.Go(fn)
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error waiting for stop functions: %w", err)
	}

	return nil
}
