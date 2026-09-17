// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package netx //nolint:testpackage // exercises the unexported cache-validity test.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The cache key is the URL, so only a ref that always resolves to the same commit
// can be served from the cache. A branch must download again every time.
func TestImmutableRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			"version tag",
			"https://github.com/sighupio/fury-distribution?ref=v1.35.1&depth=1",
			true,
		},
		{
			"version tag without the v prefix",
			"https://github.com/sighupio/installer-immutable?ref=1.35.1&depth=1",
			true,
		},
		{
			"commit sha",
			"https://github.com/sighupio/installer-immutable?ref=2870af80d9fea1dc426a65676ec79d02756fcbe7&depth=1",
			true,
		},
		{
			// The ref that made a pushed fix invisible to every later run.
			"feature branch",
			"https://github.com/sighupio/installer-immutable?ref=feat/kubernetes-1.36.4-1.35.8&depth=1",
			false,
		},
		{
			"main branch",
			"https://github.com/sighupio/installer-immutable?ref=main&depth=1",
			false,
		},
		{
			"branch that starts with a version",
			"https://github.com/sighupio/installer-immutable?ref=1.36.4-fix/lb&depth=1",
			false,
		},
		{
			"no ref at all",
			"https://github.com/sighupio/installer-immutable",
			false,
		},
		{
			"empty ref",
			"https://github.com/sighupio/installer-immutable?ref=&depth=1",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, immutableRef(test.src))
		})
	}
}
