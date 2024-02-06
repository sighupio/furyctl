// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package http_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	httpx "github.com/sighupio/furyctl/internal/x/http"
)

func TestDownloadFile(t *testing.T) {
	t.Parallel()

	fpath, err := httpx.DownloadFile("https://sighup.io")

	assert.NotNil(t, fpath)
	assert.FileExists(t, fpath)
	assert.NoError(t, err)
}
