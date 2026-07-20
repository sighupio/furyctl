// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cluster_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/stretchr/testify/require"
)

func TestStopAll_AllSucceed(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	err := cluster.StopAll(
		func() error { count.Add(1); return nil },
		func() error { count.Add(1); return nil },
		func() error { count.Add(1); return nil },
	)

	require.NoError(t, err)
	require.Equal(t, int32(3), count.Load())
}

func TestStopAll_ReturnsFirstError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")

	err := cluster.StopAll(
		func() error { return nil },
		func() error { return wantErr },
		func() error { return nil },
	)

	require.ErrorIs(t, err, wantErr)
}

func TestStopAll_MultipleErrorsNoPanicAndWaitsForAll(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	err := cluster.StopAll(
		func() error { count.Add(1); return errors.New("err1") },
		func() error { count.Add(1); return errors.New("err2") },
		func() error { count.Add(1); return errors.New("err3") },
	)

	require.Error(t, err)
	require.Equal(t, int32(3), count.Load())
}

func TestStopAll_NoFunctions(t *testing.T) {
	t.Parallel()

	require.NoError(t, cluster.StopAll())
}
