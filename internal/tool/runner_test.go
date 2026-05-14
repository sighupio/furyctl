// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tool_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool"
	itool "github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_RunnerFactory_Create(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc           string
		tool           string
		wantRunner     bool
		wantRunnerType string
	}{
		{
			desc:           "ansible",
			tool:           "ansible",
			wantRunner:     true,
			wantRunnerType: "*ansible.Runner",
		},
		{
			desc:           "furyagent",
			tool:           "furyagent",
			wantRunner:     true,
			wantRunnerType: "*furyagent.Runner",
		},
		{
			desc:           "kubectl",
			tool:           "kubectl",
			wantRunner:     true,
			wantRunnerType: "*kubectl.Runner",
		},
		{
			desc:           "kustomize",
			tool:           "kustomize",
			wantRunner:     true,
			wantRunnerType: "*kustomize.Runner",
		},
		{
			desc:           "openvpn",
			tool:           "openvpn",
			wantRunner:     true,
			wantRunnerType: "*openvpn.Runner",
		},
		{
			desc:           "terraform",
			tool:           "terraform",
			wantRunner:     true,
			wantRunnerType: "*terraform.Runner",
		},
		{
			desc:       "doesntexist",
			tool:       "doesntexist",
			wantRunner: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			rf := tool.NewRunnerFactory(execx.NewFakeExecutor("TestHelperProcess"), tool.RunnerFactoryPaths{
				Bin: os.TempDir(),
			})

			runner := rf.Create(itool.Name(tC.tool), "", os.TempDir())

			if tC.wantRunner {
				require.NotNil(t, runner)
				assert.Equal(t, tC.wantRunnerType, reflect.TypeOf(runner).String())
			} else {
				require.Nil(t, runner)
			}
		})
	}
}
