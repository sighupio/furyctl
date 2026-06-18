// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/yq"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Yq_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "4.34.1",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.34.1",
			wantErr:     true,
			wantErrMsg:  "installed = 4.34.1, expected = 1.34.1",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			yqr := tools.NewYq(newYqRunner(), tC.wantVersion)

			err := yqr.CheckBinVersion()

			if tC.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tC.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newYqRunner() *yq.Runner {
	return yq.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), yq.Paths{
		Yq: "yq",
	})
}
