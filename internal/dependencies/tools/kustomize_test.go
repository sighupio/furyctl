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
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Kustomize_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "3.9.4",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "5.6.0",
			wantErr:     true,
			wantErrMsg:  "installed = 3.9.4, expected = 5.6.0",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			fa := tools.NewKustomize(newKustomizeRunner(), tC.wantVersion)

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

func newKustomizeRunner() *kustomize.Runner {
	return kustomize.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), kustomize.Paths{
		Kustomize: "kustomize",
	})
}
