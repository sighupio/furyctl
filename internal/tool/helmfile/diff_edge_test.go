// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package helmfile_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/helmfile"
)

func TestParseHelmDiff_PlanCountsThatDoNotFitAreNotCompared(t *testing.T) {
	t.Parallel()

	stdout := "Comparing release=poc-podinfo, chart=podinfo/podinfo\n" +
		"Plan: 99999999999999999999 to add, 0 to change, 0 to destroy.\n"

	res := helmfile.ParseHelmDiff(stdout, "", 0, []string{"poc-podinfo"})

	require.Len(t, res.Releases, 1)
	require.True(t, res.Releases[0].NotCompared)
	require.Equal(t, "helmfile diff output did not match the expected format", res.Releases[0].Reason)
}
