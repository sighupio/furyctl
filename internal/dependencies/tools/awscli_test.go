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
	"github.com/sighupio/furyctl/internal/tool/awscli"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Awscli_SupportsDownload(t *testing.T) {
	a := tools.NewAwscli(newAwscliRunner(), "2.8.12")

	if a.SupportsDownload() != false {
		t.Errorf("Awscli download must not be supported")
	}
}

func Test_Awscli_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "2.8.12",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.22.0",
			wantErr:     true,
			wantErrMsg:  "installed = 2.8.12, expected = 1.22.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewAwscli(newAwscliRunner(), tC.wantVersion)

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

func newAwscliRunner() *awscli.Runner {
	return awscli.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), awscli.Paths{
		Awscli: "aws",
	})
}
