// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Validator_Validate(t *testing.T) {
	testCases := []struct {
		desc     string
		manifest config.KFD
		wantErrs []error
	}{
		{
			desc: "all tools are installed in their correct version",
			manifest: config.KFD{
				Tools: config.KFDTools{
					Common: config.Common{
						Kubectl:   config.Tool{Version: "1.21.1"},
						Kustomize: config.Tool{Version: "3.9.4"},
						Terraform: config.Tool{Version: "0.15.4"},
						Furyagent: config.Tool{Version: "0.3.0"},
					},
				},
			},
		},
		{
			desc: "all tools are installed in their wrong version",
			manifest: config.KFD{
				Tools: config.KFDTools{
					Common: config.Common{
						Kubectl:   config.Tool{Version: "1.22.0"},
						Kustomize: config.Tool{Version: "3.10.0"},
						Terraform: config.Tool{Version: "1.3.0"},
						Furyagent: config.Tool{Version: "0.4.0"},
					},
				},
			},
			wantErrs: []error{
				errors.New("furyagent: wrong tool version - installed = 0.3.0, expected = 0.4.0"),
				errors.New("kubectl: wrong tool version - installed = 1.21.1, expected = 1.22.0"),
				errors.New("kustomize: wrong tool version - installed = 3.9.4, expected = 3.10.0"),
				errors.New("terraform: wrong tool version - installed = 0.15.4, expected = 1.3.0"),
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			v := tools.NewValidator(execx.NewFakeExecutor(), "test_data")

			errs := v.Validate(tC.manifest)

			if len(errs) != len(tC.wantErrs) {
				t.Errorf("Expected %d errors, got %d - %v", len(tC.wantErrs), len(errs), errs)
			}

			for _, err := range errs {
				found := false
				for _, wantErr := range tC.wantErrs {
					if strings.Trim(err.Error(), "\n") == strings.Trim(wantErr.Error(), "\n") {
						found = true

						break
					}
				}

				if !found {
					t.Errorf("Unexpected error: %s", err)
				}
			}
		})
	}
}
