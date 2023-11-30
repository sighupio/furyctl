package distribution_test

import (
	"testing"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
)

func TestHasFeature(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		kfd  config.KFD
		want bool
	}{
		{
			desc: "v1.25 - has cluster upgrade",
			kfd: config.KFD{
				Version: "v1.25.0",
			},
			want: false,
		},
		{
			desc: "v1.26 - has cluster upgrade",
			kfd: config.KFD{
				Version: "v1.26.0",
			},
			want: true,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			got := distribution.HasFeature(tC.kfd, distribution.FeatureClusterUpgrade)

			if got != tC.want {
				t.Errorf("got %t, want %t", got, tC.want)
			}
		})
	}
}
