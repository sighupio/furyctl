// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package app_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/app"
)

func Test_Update_FetchLastRelease(t *testing.T) {
	got, err := app.GetLatestRelease()
	if err != nil {
		t.Fatal(err)
	}

	if got.Version == "" {
		t.Error("Version is empty")
	}

	if got.URL == "" {
		t.Error("Version is empty")
	}
}
