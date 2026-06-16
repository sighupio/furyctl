// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Ansible_SupportsDownload(t *testing.T) {
	a := tools.NewAnsible(newAnsibleRunner(), "v0.2.0")

	if !a.SupportsDownload() {
		t.Errorf("Ansible download should be supported")
	}
}

func Test_Ansible_SrcPath(t *testing.T) {
	a := tools.NewAnsible(newAnsibleRunner(), "v0.2.0")

	want := fmt.Sprintf(
		"https://github.com/nutellinoit/ansible-portable-poc/releases/download/"+
			"v0.2.0/ansible-portable-v0.2.0-%s-%s.tar.gz",
		runtime.GOOS,
		runtime.GOARCH,
	)

	if a.SrcPath() != want {
		t.Errorf("Wrong ansible src path: want = %s, got = %s", want, a.SrcPath())
	}
}

func Test_Ansible_Rename(t *testing.T) {
	a := tools.NewAnsible(newAnsibleRunner(), "v0.2.0")

	if a.Rename("") != nil {
		t.Errorf("Ansible rename never returns errors")
	}
}

func Test_Ansible_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		runner     *ansible.Runner
		wantErr    bool
		wantErrMsg string
	}{
		{
			// Presence only: the bundle binary runs, so no error regardless of the reported
			// ansible-core version (the configured version is the bundle release tag).
			desc:   "present and runs",
			runner: newAnsibleRunner(),
		},
		{
			desc:       "missing binary",
			runner:     newMissingAnsibleRunner(),
			wantErr:    true,
			wantErrMsg: "missing binary",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			a := tools.NewAnsible(tC.runner, "v0.2.0")

			err := a.CheckBinVersion()

			if tC.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tC.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newAnsibleRunner() *ansible.Runner {
	return ansible.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), ansible.Paths{
		Ansible:         "ansible",
		AnsiblePlaybook: "ansible-playbook",
	})
}

func newMissingAnsibleRunner() *ansible.Runner {
	return ansible.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), ansible.Paths{
		Python:          "/this/path/does/not/exist/python/bin/python3",
		Ansible:         "/this/path/does/not/exist/python/bin/ansible",
		AnsiblePlaybook: "/this/path/does/not/exist/python/bin/ansible-playbook",
	})
}
