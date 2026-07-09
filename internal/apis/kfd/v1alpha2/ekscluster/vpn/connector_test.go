// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package vpn_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
)

func TestConnector_ValidateConfig(t *testing.T) {
	t.Parallel()

	intPtr := func(i int) *int { return &i }

	testCases := []struct {
		desc        string
		autoConnect bool
		config      *private.SpecInfrastructureVpn
		wantErr     bool
	}{
		{
			desc:        "no auto-connect and no vpn is valid",
			autoConnect: false,
			config:      nil,
		},
		{
			desc:        "no auto-connect with vpn configured is valid",
			autoConnect: false,
			config:      &private.SpecInfrastructureVpn{Instances: intPtr(1)},
		},
		{
			desc:        "auto-connect with vpn (default instances) is valid",
			autoConnect: true,
			config:      &private.SpecInfrastructureVpn{},
		},
		{
			desc:        "auto-connect with positive vpn instances is valid",
			autoConnect: true,
			config:      &private.SpecInfrastructureVpn{Instances: intPtr(2)},
		},
		{
			desc:        "auto-connect without any vpn configuration is rejected",
			autoConnect: true,
			config:      nil,
			wantErr:     true,
		},
		{
			desc:        "auto-connect with zero vpn instances is rejected",
			autoConnect: true,
			config:      &private.SpecInfrastructureVpn{Instances: intPtr(0)},
			wantErr:     true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			connector, err := vpn.NewConnector(
				"test-cluster",
				"cert-dir",
				"bin-path",
				"0.3.0",
				tC.autoConnect,
				false,
				tC.config,
			)
			require.NoError(t, err)

			err = connector.ValidateConfig()

			if tC.wantErr {
				require.ErrorIs(t, err, vpn.ErrAutoConnectWithoutVpn)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
