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

func Test_ExtraToolsValidator_openVPN(t *testing.T) {
	t.Parallel()

	intPtr := func(i int) *int { return &i }

	vpnConf := func(instances *int) private.EksclusterKfdV1Alpha2 {
		return private.EksclusterKfdV1Alpha2{
			Spec: private.Spec{
				Infrastructure: &private.SpecInfrastructure{
					Vpn: &private.SpecInfrastructureVpn{
						Instances: instances,
					},
				},
			},
		}
	}

	noVPNInfra := private.EksclusterKfdV1Alpha2{
		Spec: private.Spec{Infrastructure: &private.SpecInfrastructure{}},
	}

	noInfra := private.EksclusterKfdV1Alpha2{
		Spec: private.Spec{},
	}

	testCases := []struct {
		desc           string
		conf           private.EksclusterKfdV1Alpha2
		openvpnMissing bool
		wantErr        bool
	}{
		{
			desc: "vpn enabled with default instances and openvpn installed",
			conf: vpnConf(nil),
		},
		{
			desc:           "vpn enabled with default instances and openvpn missing (no auto-connect)",
			conf:           vpnConf(nil),
			openvpnMissing: true,
			wantErr:        true,
		},
		{
			desc:           "vpn enabled with positive instances and openvpn missing",
			conf:           vpnConf(intPtr(2)),
			openvpnMissing: true,
			wantErr:        true,
		},
		{
			desc:           "vpn disabled with zero instances does not require openvpn",
			conf:           vpnConf(intPtr(0)),
			openvpnMissing: true,
		},
		{
			desc:           "no vpn configuration does not require openvpn",
			conf:           noVPNInfra,
			openvpnMissing: true,
		},
		{
			desc:           "no infrastructure does not require openvpn",
			conf:           noInfra,
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

			err := validator.openVPN(tC.conf)

			if tC.wantErr {
				require.ErrorIs(t, err, ErrOpenVPNNotInstalled)
			} else {
				require.NoError(t, err)
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
		fmt.Fprint(os.Stdout, "OpenVPN 2.5.7 x86_64-pc-linux-gnu [SSL (OpenSSL)] [LZO] [LZ4]\n")
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
