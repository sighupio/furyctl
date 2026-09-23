// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package test_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	test "github.com/sighupio/furyctl/test/e2e/utils"
)

func TestNewS3KeyPrefix(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for range 10000 {
		got := test.NewS3KeyPrefix()

		assert.Len(t, got, 36)
		assert.False(t, seen[got], "duplicate key prefix %s", got)

		seen[got] = true
	}
}
