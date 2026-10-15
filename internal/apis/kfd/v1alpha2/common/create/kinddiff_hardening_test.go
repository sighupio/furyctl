// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

func TestRunDiff_PluginsRenderErrorFailsEveryPluginsSection(t *testing.T) {
	t.Parallel()

	r := &recorder{exists: true}

	in := r.input("")
	in.DiffPlugins = func() ([]create.Section, error) {
		return nil, errors.New("missing\n  key")
	}

	sections := create.RunDiff(in)

	require.Equal(t,
		"Distribution Compared: ; Plugins (kustomize) Failed: render failed: missing key; "+
			"Plugins (helm) Failed: render failed: missing key",
		describeSections(sections))
}
