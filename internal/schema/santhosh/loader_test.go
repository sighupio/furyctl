// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package santhosh_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

func TestLoadSchema(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		schemaPath string
		wantErr    bool
		wantErrMsg string
	}{
		{
			desc:       "not existing schema",
			schemaPath: "not-existing-schema.json",
			wantErr:    true,
			wantErrMsg: "no such file or directory",
		},
		{
			desc:       "broken schema",
			schemaPath: "../../../test/data/integration/schema/santhosh/test-cluster-broken.json",
			wantErr:    true,
			wantErrMsg: "jsonschema: invalid json ../../../test/data/integration/schema/santhosh/test-cluster-broken.json: unexpected EOF",
		},
		{
			desc:       "wrong schema",
			schemaPath: "../../../test/data/integration/schema/santhosh/test-cluster-wrong.json",
			wantErr:    true,
			wantErrMsg: "compilation failed",
		},
		{
			desc:       "minimal correct schema",
			schemaPath: "../../../test/data/integration/schema/santhosh/test-cluster-correct.json",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			s, err := santhosh.LoadSchema(tC.schemaPath)

			if tC.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, santhosh.ErrCannotLoadSchema)
				assert.Contains(t, err.Error(), tC.wantErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, s)
			}
		})
	}
}
