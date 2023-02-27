// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks"
)

func Test_ExtraSchemaValidator_Validate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		confPath   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			desc:     "min size is lesser than max size",
			confPath: "test/schema/min_lesser_than_max.yaml",
		},
		{
			desc:     "min size is equal to max size",
			confPath: "test/schema/min_equal_to_max.yaml",
		},
		{
			desc:       "min size is greater than max size",
			confPath:   "test/schema/min_greater_than_max.yaml",
			wantErr:    true,
			wantErrMsg: "invalid node pool size: element 0's max size(1) must be greater than or equal to its min(2)",
		},
		{
			desc:     "furyctl config is invalid",
			confPath: "test/schema/invalid.yaml",
			wantErr:  true,
			wantErrMsg: "error while unmarshalling file from test/schema/invalid.yaml" +
				" :yaml: line 1: did not find expected ',' or '}'",
		},
	}

	esv := &eks.ExtraSchemaValidator{}

	for _, tC := range testCases {
		tC := tC
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			err := esv.Validate(tC.confPath)

			if tC.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tC.wantErr && err != nil {
				t.Errorf("expected nil, got error: %v", err)
			}

			if tC.wantErr && err != nil && err.Error() != tC.wantErrMsg {
				t.Errorf("expected error message '%s', got '%s'", tC.wantErrMsg, err.Error())
			}
		})
	}
}
