// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Openvpn_SupportsDownload(t *testing.T) {
	t.Parallel()

	a := tools.NewOpenvpn(newOpenvpnRunner(), "2.5.7")

	if a.SupportsDownload() != false {
		t.Errorf("Openvpn download is not supported")
	}
}

func Test_Openvpn_SrcPath(t *testing.T) {
	t.Parallel()

	a := tools.NewOpenvpn(newOpenvpnRunner(), "2.5.7")

	if a.SrcPath() != "" {
		t.Errorf("Wrong openvpn src path: want = %s, got = %s", "", a.SrcPath())
	}
}

func Test_Openvpn_Rename(t *testing.T) {
	t.Parallel()

	a := tools.NewOpenvpn(newOpenvpnRunner(), "2.5.7")

	if a.Rename("") != nil {
		t.Errorf("Openvpn rename never returns errors")
	}
}

func Test_Openvpn_CheckBinVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		wantVersion string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			desc:        "correct version installed",
			wantVersion: "2.5.7",
		},
		{
			desc:        "wrong version installed",
			wantVersion: "2.4.0",
			wantErr:     true,
			wantErrMsg:  "installed = 2.5.7, expected = 2.4.0",
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			err := tools.NewOpenvpn(newOpenvpnRunner(), tC.wantVersion).CheckBinVersion()

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

func newOpenvpnRunner() *openvpn.Runner {
	return openvpn.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), openvpn.Paths{
		Openvpn: "openvpn",
	})
}
