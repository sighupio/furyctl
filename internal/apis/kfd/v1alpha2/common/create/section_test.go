// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

func TestSectionGroup(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Distribution":           "",
		"Plugin kustomize/alpha": create.KustomizeGroup,
		create.KustomizeGroup:    create.KustomizeGroup,
		"Plugin helm/podinfo":    create.HelmGroup,
		create.HelmGroup:         create.HelmGroup,
	}

	for title, group := range tests {
		require.Equal(t, group, create.Section{Title: title}.Group(), title)
	}
}
