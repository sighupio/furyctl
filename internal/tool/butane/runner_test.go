// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package butane_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/butane"
)

// An XFS filesystem with a label longer than 12 characters passes source
// validation but fails validation of the generated Ignition config, so Butane
// reports the reason in the report and not in the error.
const invalidXFSLabelConfig = `variant: flatcar
version: 1.1.0
storage:
  filesystems:
    - device: /dev/disk/by-partlabel/data
      format: xfs
      label: way-too-long-label
`

func TestRunner_Convert_SurfacesValidationDetails(t *testing.T) {
	t.Parallel()

	_, err := butane.NewRunner().Convert([]byte(invalidXFSLabelConfig))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config generated was invalid")
	assert.Contains(t, err.Error(), "$.storage.filesystems.0.label")
	assert.Contains(t, err.Error(), "cannot be longer than 12 characters")
}

func TestRunner_ConvertWithReport_SurfacesValidationDetails(t *testing.T) {
	t.Parallel()

	_, _, err := butane.NewRunner().ConvertWithReport([]byte(invalidXFSLabelConfig))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "$.storage.filesystems.0.label")
	assert.Contains(t, err.Error(), "cannot be longer than 12 characters")
}

func TestRunner_Convert_KeepsBareErrorWithoutReport(t *testing.T) {
	t.Parallel()

	// No variant: Butane fails before it produces a report.
	_, err := butane.NewRunner().Convert([]byte("version: 1.1.0\n"))
	require.Error(t, err)
	assert.Equal(t, "error translating butane config: error parsing variant; must be specified", err.Error())
}
