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
	"github.com/sighupio/furyctl/internal/tool/kapp"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Kapp_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErrMsg  string
		wantErr     bool
	}{
		{
			desc:        "correct version installed",
			wantVersion: "0.62.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "3.5.3",
			wantErr:     true,
			wantErrMsg:  "installed = 0.62.0, expected = 3.5.3",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewKapp(newKappRunner(), tC.wantVersion)

			err := fa.CheckBinVersion()

			if tC.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tC.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newKappRunner() *kapp.Runner {
	return kapp.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), kapp.Paths{
		Kapp: "kapp",
	}, false)
}
