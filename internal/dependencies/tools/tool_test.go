// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/sighupio/furyctl/internal/dependencies/tools"
	itool "github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Factory_Create(t *testing.T) {
	testCases := []struct {
		desc     string
		wantTool bool
	}{
		{
			desc:     "furyagent",
			wantTool: true,
		},
		{
			desc:     "kubectl",
			wantTool: true,
		},
		{
			desc:     "kustomize",
			wantTool: true,
		},
		{
			desc:     "openvpn",
			wantTool: true,
		},
		{
			desc:     "terraform",
			wantTool: true,
		},
		{
			desc:     "yq",
			wantTool: true,
		},
		{
			desc:     "shell",
			wantTool: true,
		},
		{
			desc:     "unsupported",
			wantTool: false,
		},
	}
	for _, tC := range testCases {
		f := tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{
			Bin: "",
		})
		t.Run(tC.desc, func(t *testing.T) {
			tool := f.Create(itool.Name(tC.desc), "0.0.0")
			if tool == nil && tC.wantTool {
				t.Errorf("Expected tool %s, got nil", tC.desc)
			}
			if tool != nil && !tC.wantTool {
				t.Errorf("Expected nil, got tool %s", tC.desc)
			}
		})
	}
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "ansible":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "ansible [core 2.9.27]\n  "+
				"config file = None\n  "+
				"configured module search path = ['', '']\n"+
				"ansible python module location = ./ansible\n"+
				"ansible collection location = ./ansible/collections\n"+
				"executable location = ./bin/ansible\n  "+
				"python version = 3.9.14\n"+
				"jinja version = 3.1.2\n"+
				"libyaml = True\n")
		}
	case "aws":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "aws-cli/2.8.12 Python/3.11.0 Darwin/21.6.0 source/arm64 prompt/off\n")
		case "s3api":
			fmt.Fprintf(os.Stdout, "eu-west-1\n")
		}
	case "furyagent":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "Furyagent version 0.3.0 - md5: b7d2b3dc7398ac6ce120a17d4fd439f2 - /opt/homebrew/bin/furyagent")
		}
	case "git":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "git version 2.39.0\n")
		}
	case "kubectl":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "{\n"+
				"\"clientVersion\": {\n"+
				"\"major\": \"1\",\n"+
				"\"minor\": \"21\",\n"+
				"\"gitVersion\": \"v1.21.1\",\n"+
				"\"gitCommit\": \"xxxxx\",\n"+
				"\"gitTreeState\": \"clean\",\n"+
				"\"buildDate\": \"2021-05-12T14:00:00Z\",\n"+
				"\"goVersion\": \"go1.16.4\",\n"+
				"\"compiler\": \"gc\",\n"+
				"\"platform\": \"darwin/amd64\"\n"+
				"}\n"+
				"}\n")
		}
	case "kustomize":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "Version: {kustomize/v3.9.4 GitCommit:xxxxxxx"+
				"BuildDate:2021-05-12T14:00:00Z GoOs:darwin GoArch:amd64}")
		}
	case "openvpn":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "OpenVPN 2.5.7 arm-apple-darwin21.5.0 [SSL (OpenSSL)] [LZO] [LZ4] [PKCS11] [MH/RECVDA] [AEAD] built on Jun  8 2022\n"+
				"library versions: OpenSSL 1.1.1q  5 Jul 2022, LZO 2.10\n"+
				"Originally developed by James Yonan\n"+
				"Copyright (C) 2002-2022 OpenVPN Inc <sales@openvpn.net>\n"+
				"Compile time defines: enable_async_push=no enable_comp_stub=no enable_crypto_ofb_cfb=yes enable_debug=no enable_def_auth=yes enable_dependency_tracking=no enable_dlopen=unknown enable_dlopen_self=unknown enable_dlopen_self_static=unknown enable_fast_install=needless enable_fragment=yes enable_iproute2=no enable_libtool_lock=yes enable_lz4=yes enable_lzo=yes enable_management=yes enable_multihome=yes enable_pam_dlopen=no enable_pedantic=no enable_pf=yes enable_pkcs11=yes enable_plugin_auth_pam=yes enable_plugin_down_root=yes enable_plugins=yes enable_port_share=yes enable_selinux=no enable_shared=yes enable_shared_with_static_runtimes=no enable_silent_rules=no enable_small=no enable_static=yes enable_strict=no enable_strict_options=no enable_systemd=no enable_werror=no enable_win32_dll=yes enable_x509_alt_username=no with_aix_soname=aix with_crypto_library=openssl with_gnu_ld=no with_mem_check=no with_openssl_engine=auto with_sysroot=no\n")
		}
	case "terraform":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "Terraform v0.15.4\non darwin_amd64")
		}
	case "yq":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "yq (https://github.com/mikefarah/yq/) version v4.34.1")
		}
	case "sh":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "GNU bash, version 3.2.57(1)-release (arm64-apple-darwin22)\nCopyright (C) 2007 Free Software Foundation, Inc.")
		}
	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}
