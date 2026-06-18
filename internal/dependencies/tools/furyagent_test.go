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
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Furyagent_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "0.3.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "0.4.0",
			wantErr:     true,
			wantErrMsg:  "installed = 0.3.0, expected = 0.4.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewFuryagent(newFuryagentRunner(), tC.wantVersion)

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

func newFuryagentRunner() *furyagent.Runner {
	return furyagent.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), furyagent.Paths{
		Furyagent: "furyagent",
	})
}
