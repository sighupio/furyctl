// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Ansible_SupportsDownload(t *testing.T) {
	a := tools.NewAnsible(newAnsibleRunner(), "2.9.27")

	if a.SupportsDownload() != false {
		t.Errorf("Ansible download is not supported")
	}
}

func Test_Ansible_SrcPath(t *testing.T) {
	a := tools.NewAnsible(newAnsibleRunner(), "2.9.27")

	if a.SrcPath() != "" {
		t.Errorf("Wrong ansible src path: want = %s, got = %s", "", a.SrcPath())
	}
}

func Test_Ansible_Rename(t *testing.T) {
	a := tools.NewAnsible(newAnsibleRunner(), "2.9.27")

	if a.Rename("") != nil {
		t.Errorf("Ansible rename never returns errors")
	}
}

func Test_Ansible_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "2.9.27",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "2.10.0",
			wantErr:     true,
			wantErrMsg:  "installed = 2.9.27, expected = 2.10.0",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			a := tools.NewAnsible(newAnsibleRunner(), tC.wantVersion)

			err := a.CheckBinVersion()

			if tC.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tC.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if tC.wantErr && err != nil && !strings.Contains(err.Error(), tC.wantErrMsg) {
				t.Errorf("expected error message '%s' to contain '%s'", err.Error(), tC.wantErrMsg)
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
