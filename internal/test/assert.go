// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func AssertErrorIs(t *testing.T, err, want error) {
	t.Helper()

	if want == nil {
		require.NoError(t, err)
	} else {
		require.ErrorIs(t, err, want)
	}
}
