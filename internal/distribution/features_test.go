// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution_test

import (
	"testing"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
)

func TestHasFeature(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		kfd     config.KFD
		feature distribution.Feature
		want    bool
	}{
		{
			desc: "v1.25 - has cluster upgrade",
			kfd: config.KFD{
				Version: "v1.25.0",
			},
			feature: distribution.FeatureClusterUpgrade,
			want:    false,
		},
		{
			desc: "v1.26 - has cluster upgrade",
			kfd: config.KFD{
				Version: "v1.26.4",
			},
			feature: distribution.FeatureClusterUpgrade,
			want:    true,
		},
		{
			desc: "v1.28 - has kapp support",
			kfd: config.KFD{
				Version: "v1.28.0",
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kapp: config.KFDTool{
							Version: "1.2.3",
						},
					},
				},
			},
			feature: distribution.FeatureKappSupport,
			want:    true,
		},
		{
			desc: "v1.29 - has kapp support - empty kapp",
			kfd: config.KFD{
				Version: "v1.29.0",
			},
			feature: distribution.FeatureKappSupport,
			want:    false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := distribution.HasFeature(tC.kfd, tC.feature)

			if got != tC.want {
				t.Errorf("got %t, want %t", got, tC.want)
			}
		})
	}
}
