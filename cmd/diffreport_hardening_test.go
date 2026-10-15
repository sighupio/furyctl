// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd //nolint:testpackage // exercises the unexported report builder.

import (
	"testing"

	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

func TestResourceLines_ClusterScopedFirstEvenWhenANamespacedAddressSortsBefore(t *testing.T) {
	t.Parallel()

	lines := resourceLines(commcreate.Section{Rows: []commcreate.Row{
		{Namespace: "Alpha", Kind: "ConfigMap", Name: "a", Op: opCreate},
		{Namespace: clusterScoped, Kind: "Namespace", Name: "demo", Op: opCreate},
	}})

	require.Equal(t, "+ Namespace/demo", lines[0].text)
	require.Equal(t, "+ Alpha/ConfigMap/a", lines[1].text)
}

func TestGroupPluginSections_DifferentStatusesWithTheSameReasonAreNotSummarized(t *testing.T) {
	t.Parallel()

	grouped := groupPluginSections([]commcreate.Section{
		{Title: "Plugin kustomize/a", Status: commcreate.StatusCompared},
		{Title: "Plugin kustomize/b", Status: commcreate.StatusFailed},
	})

	require.Len(t, grouped, 2)
	require.Equal(t, "Plugin kustomize/a", grouped[0].Title)
	require.Equal(t, "Plugin kustomize/b", grouped[1].Title)
}
