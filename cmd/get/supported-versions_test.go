// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd/get"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/mocks"
)

func TestFormatDistroVersions(t *testing.T) {
	t.Parallel()

	mockGhClient := mocks.NewMockGitHubClient(
		[]git.Release{{
			TagName:     "v1.20.0",
			CreatedAt:   time.Date(2020, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2020, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.22.0",
			CreatedAt:   time.Date(2021, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2021, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.25.0",
			CreatedAt:   time.Date(2022, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2022, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.27.0",
			CreatedAt:   time.Date(2022, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2020, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.94.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.96.0",
			CreatedAt:   time.Date(2023, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2023, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.97.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.98.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}, {
			TagName:     "v1.99.0",
			CreatedAt:   time.Date(2025, time.February, 6, 12, 30, 0, 0, time.UTC),
			PublishedAt: time.Date(2025, time.February, 7, 12, 30, 0, 0, time.UTC),
			PreRelease:  false,
		}},
	)

	releases, err := distribution.GetSupportedVersions(mockGhClient)

	require.NoError(t, err)

	fmtString := get.FormatSupportedVersions(releases, []string{distribution.EKSClusterKind, distribution.KFDDistributionKind, distribution.OnPremisesKind})
	lines := strings.Split(fmtString, "\n")

	assert.Equal(t, "-----------------------------------------------------------------------------------------", lines[2])
	assert.Equal(t, "VERSION \t\tRELEASE DATE\t\tEKSCluster\tKFDDistribution\tOnPremises\t", lines[3])
	assert.Equal(t, "-----------------------------------------------------------------------------------------", lines[4])
	assert.Contains(t, lines[5], "v1.99.0*\t\t2025-02-06")
	assert.Contains(t, lines[6], "v1.98.0*\t\t2025-02-06")
	assert.Contains(t, lines[7], "v1.97.0*\t\t2025-02-06")
	assert.Equal(t, "* this usually indicates you are not using the latest version of furyctl, try updating or checking the online documentation.", lines[8])
}
