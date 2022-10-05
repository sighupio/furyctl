// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package santhosh_test

import (
	"errors"
	"strings"
	"testing"

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
			wantErrMsg: "parsing \"../../../test/data/integration/schema/santhosh/test-cluster-broken.json\" failed",
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
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			s, err := santhosh.LoadSchema(tC.schemaPath)

			if !tC.wantErr && err != nil {
				t.Errorf("want no error, got %v", err)
			}

			if tC.wantErr && err == nil {
				t.Errorf("want error, got none")
			}

			if tC.wantErr && err != nil && !errors.Is(err, santhosh.ErrCannotLoadSchema) {
				t.Errorf("want error %v, got %v", santhosh.ErrCannotLoadSchema, err)
			}

			if tC.wantErr && err != nil && !strings.Contains(err.Error(), tC.wantErrMsg) {
				t.Errorf("want error message '%s' to contain '%s'", err.Error(), tC.wantErrMsg)
			}

			if !tC.wantErr && s == nil {
				t.Errorf("want schema, got nil")
			}
		})
	}
}
