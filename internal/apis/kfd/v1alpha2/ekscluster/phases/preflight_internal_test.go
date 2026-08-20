// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package phases

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
)

// TestPreFlight_getOperatorName covers the default that furyagent uses to reach the
// VPN bastion: an absent .spec.infrastructure.vpn.operatorName falls back to
// "sighup", and a value in the configuration wins.
func TestPreFlight_getOperatorName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc         string
		operatorName *string
		want         string
	}{
		{
			desc:         "the configuration has no operator name",
			operatorName: nil,
			want:         "sighup",
		},
		{
			desc:         "the configuration has an operator name",
			operatorName: lo.ToPtr("ubuntu"),
			want:         "ubuntu",
		},
		{
			desc:         "the configuration has an empty operator name",
			operatorName: lo.ToPtr(""),
			want:         "",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			p := &PreFlight{
				FuryctlConf: private.EksclusterKfdV1Alpha2{
					Spec: private.Spec{
						Infrastructure: &private.SpecInfrastructure{
							Vpn: &private.SpecInfrastructureVpn{
								OperatorName: tC.operatorName,
							},
						},
					},
				},
			}

			assert.Equal(t, tC.want, p.getOperatorName())
		})
	}
}
