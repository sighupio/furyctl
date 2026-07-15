// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package ekscluster

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func vpnConf(instances int) private.EksclusterKfdV1Alpha2 {
	return private.EksclusterKfdV1Alpha2{
		Spec: private.Spec{
			Infrastructure: &private.SpecInfrastructure{
				Vpn: &private.SpecInfrastructureVpn{
					Instances: &instances,
				},
			},
		},
	}
}

func vpnConfDefault() private.EksclusterKfdV1Alpha2 {
	return private.EksclusterKfdV1Alpha2{
		Spec: private.Spec{
			Infrastructure: &private.SpecInfrastructure{
				Vpn: &private.SpecInfrastructureVpn{},
			},
		},
	}
}

func noVPNInfra() private.EksclusterKfdV1Alpha2 {
	return private.EksclusterKfdV1Alpha2{
		Spec: private.Spec{Infrastructure: &private.SpecInfrastructure{}},
	}
}

func noInfra() private.EksclusterKfdV1Alpha2 {
	return private.EksclusterKfdV1Alpha2{
		Spec: private.Spec{},
	}
}

func Test_ExtraToolsValidator_openVPN(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc  string
		fails bool
	}{
		{
			desc: "openvpn installed returns no error",
		},
		{
			desc:  "openvpn missing returns error",
			fails: true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			helper := "TestHelperProcessOpenvpnOK"
			if tC.fails {
				helper = "TestHelperProcessOpenvpnMissing"
			}

			validator := NewExtraToolsValidator(execx.NewFakeExecutor(helper))

			err := validator.openVPN()

			if tC.fails {
				require.ErrorIs(t, err, ErrOpenVPNNotInstalled)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ExtraToolsValidator_validate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc           string
		conf           private.EksclusterKfdV1Alpha2
		openvpnMissing bool
		wantOks        []string
		wantErr        bool
	}{
		{
			desc:    "vpn enabled with default instances and openvpn installed adds openvpn to oks",
			conf:    vpnConfDefault(),
			wantOks: []string{"openvpn"},
		},
		{
			desc:           "vpn enabled with default instances and openvpn missing reports error and no oks",
			conf:           vpnConfDefault(),
			openvpnMissing: true,
			wantErr:        true,
		},
		{
			desc:           "vpn enabled with positive instances and openvpn missing reports error and no oks",
			conf:           vpnConf(2),
			openvpnMissing: true,
			wantErr:        true,
		},
		{
			desc:           "vpn disabled with zero instances does not validate openvpn",
			conf:           vpnConf(0),
			openvpnMissing: true,
		},
		{
			desc:           "no vpn configuration does not validate openvpn",
			conf:           noVPNInfra(),
			openvpnMissing: true,
		},
		{
			desc:           "no infrastructure does not validate openvpn",
			conf:           noInfra(),
			openvpnMissing: true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			helper := "TestHelperProcessOpenvpnOK"
			if tC.openvpnMissing {
				helper = "TestHelperProcessOpenvpnMissing"
			}

			validator := NewExtraToolsValidator(execx.NewFakeExecutor(helper))

			oks, errs := validator.validateConf(tC.conf)

			require.Equal(t, tC.wantOks, oks)

			if tC.wantErr {
				require.Len(t, errs, 1)
				require.ErrorIs(t, errs[0], ErrOpenVPNNotInstalled)
			} else {
				require.Empty(t, errs)
			}
		})
	}
}

// TestHelperProcessOpenvpnOK simulates a working `openvpn --version` invocation when spawned as a
// subprocess by the fake executor; it is a no-op during a normal test run.
func TestHelperProcessOpenvpnOK(t *testing.T) {
	if len(os.Args) < 5 || os.Args[1] != "-test.run=TestHelperProcessOpenvpnOK" {
		return
	}

	if os.Args[3] == "openvpn" && os.Args[4] == "--version" {
		fmt.Fprint(os.Stdout, "OpenVPN 2.7.5 aarch64-apple-darwin25.4.0 [SSL (OpenSSL)] [LZO] [LZ4] [PKCS11] [MH/RECVDA] [AEAD]\n"+
			"library versions: OpenSSL 3.5.0 8 Apr 2025, LZO 2.10\n")
	}
}

// TestHelperProcessOpenvpnMissing simulates `exec: "openvpn": executable file not found in $PATH`
// by exiting non-zero when spawned as a subprocess; it is a no-op during a normal test run.
func TestHelperProcessOpenvpnMissing(t *testing.T) {
	if len(os.Args) < 2 || os.Args[1] != "-test.run=TestHelperProcessOpenvpnMissing" {
		return
	}

	os.Exit(1)
}
