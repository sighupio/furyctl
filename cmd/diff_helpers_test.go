// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd //nolint:testpackage // exercises unexported helpers.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiffLines(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a", "b"}, diffLines("a\n\nb\n"))
	require.Empty(t, diffLines(""))
}

func TestSkippableStep(t *testing.T) {
	t.Parallel()

	errStep := errors.New("step failed")
	calls := 0
	step := func() error {
		calls++

		return errStep
	}

	require.NoError(t, skippableStep{skip: true, doing: "doing", skipped: "skipped", run: step}.exec())
	require.Equal(t, 0, calls)

	require.ErrorIs(t, skippableStep{doing: "doing", skipped: "skipped", run: step}.exec(), errStep)
	require.Equal(t, 1, calls)
}
