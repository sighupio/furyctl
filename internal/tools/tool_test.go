// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/tools"
)

func Test_Factory_Create(t *testing.T) {
	testCases := []struct {
		desc     string
		wantTool bool
	}{
		{
			desc:     "furyagent",
			wantTool: true,
		},
		{
			desc:     "kubectl",
			wantTool: true,
		},
		{
			desc:     "kustomize",
			wantTool: true,
		},
		{
			desc:     "terraform",
			wantTool: true,
		},
		{
			desc:     "unsupported",
			wantTool: false,
		},
	}
	for _, tC := range testCases {
		f := tools.NewFactory()
		t.Run(tC.desc, func(t *testing.T) {
			tool := f.Create(tC.desc, "0.0.0")
			if tool == nil && tC.wantTool {
				t.Errorf("Expected tool %s, got nil", tC.desc)
			}
			if tool != nil && !tC.wantTool {
				t.Errorf("Expected nil, got tool %s", tC.desc)
			}
		})
	}
}
