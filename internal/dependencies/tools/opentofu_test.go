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
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_OpenTofu_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "1.10.0",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "1.8.0",
			wantErr:     true,
			wantErrMsg:  "installed = 1.10.0, expected = 1.8.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewOpenTofu(newOpenTofuRunner(), tC.wantVersion)

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

func newOpenTofuRunner() *terraform.Runner {
	return terraform.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), terraform.Paths{
		Terraform: "tofu",
	})
}
