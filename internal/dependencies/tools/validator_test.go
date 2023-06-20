// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Validator_Validate(t *testing.T) {
	testCases := []struct {
		desc     string
		manifest config.KFD
		state    config.Furyctl
		wantOks  []string
		wantErrs []error
	}{
		{
			desc: "all tools are installed in their correct version",
			manifest: config.KFD{
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.21.1"},
						Kustomize: config.KFDTool{Version: "3.9.4"},
						Terraform: config.KFDTool{Version: "0.15.4"},
						Furyagent: config.KFDTool{Version: "0.3.0"},
						Yq:        config.KFDTool{Version: "4.34.1"},
					},
				},
			},
			state: config.Furyctl{
				Spec: config.FuryctlSpec{},
			},
			wantOks: []string{
				"kubectl",
				"kustomize",
				"terraform",
				"furyagent",
				"yq",
			},
		},
		{
			desc: "all tools are installed in their wrong version",
			manifest: config.KFD{
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.22.0"},
						Kustomize: config.KFDTool{Version: "3.5.3"},
						Terraform: config.KFDTool{Version: "1.3.0"},
						Furyagent: config.KFDTool{Version: "0.4.0"},
						Yq:        config.KFDTool{Version: "4.33.0"},
					},
				},
			},
			state: config.Furyctl{
				Spec: config.FuryctlSpec{},
			},
			wantErrs: []error{
				errors.New("furyagent: wrong tool version - installed = 0.3.0, expected = 0.4.0"),
				errors.New("kubectl: wrong tool version - installed = 1.21.1, expected = 1.22.0"),
				errors.New("kustomize: wrong tool version - installed = 3.9.4, expected = 3.5.3"),
				errors.New("terraform: wrong tool version - installed = 0.15.4, expected = 1.3.0"),
				errors.New("yq: wrong tool version - installed = 4.34.1, expected = 4.33.0"),
			},
		},
		{
			desc: "all tools for EKSCluster kind are installed",
			manifest: config.KFD{
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.21.1"},
						Kustomize: config.KFDTool{Version: "3.9.4"},
						Terraform: config.KFDTool{Version: "0.15.4"},
						Furyagent: config.KFDTool{Version: "0.3.0"},
						Yq:        config.KFDTool{Version: "4.34.1"},
					},
					Eks: config.KFDToolsEks{
						Awscli: config.KFDTool{Version: "2.8.12"},
					},
				},
			},
			state: config.Furyctl{
				APIVersion: "kfd.sighup.io/v1alpha2",
				Kind:       "EKSCluster",
				Spec:       config.FuryctlSpec{},
			},
			wantOks: []string{
				"kubectl",
				"kustomize",
				"terraform",
				"furyagent",
				"yq",
				"awscli",
				"openvpn",
				"terraform state aws s3 bucket",
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			furyctlPath := path.Join("test_data", "furyctl.yaml")

			v := tools.NewValidator(execx.NewFakeExecutor("TestHelperProcess"), "test_data", furyctlPath, false)

			oks, errs := v.Validate(tC.manifest, tC.state)

			if len(oks) != len(tC.wantOks) {
				t.Errorf("Expected %d oks, got %d - %v", len(tC.wantOks), len(oks), oks)
			}

			if len(errs) != len(tC.wantErrs) {
				t.Errorf("Expected %d errors, got %d - %v", len(tC.wantErrs), len(errs), errs)
			}

			for _, ok := range oks {
				found := false
				for _, wantOk := range tC.wantOks {
					if ok == wantOk {
						found = true

						break
					}
				}

				if !found {
					t.Errorf("Unexpected ok: %s", ok)
				}
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

func TestValidator_ValidateBaseReqs(t *testing.T) {
	testCases := []struct {
		desc     string
		wantOks  []string
		wantErrs []error
	}{
		{
			desc: "all base requirements are met",
			wantOks: []string{
				"git",
				"shell",
			},
		},
	}

	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			furyctlPath := path.Join("test_data", "furyctl.yaml")

			v := tools.NewValidator(execx.NewFakeExecutor("TestHelperProcess"), "test_data", furyctlPath, false)

			oks, errs := v.ValidateBaseReqs()

			if len(oks) != len(tC.wantOks) {
				t.Errorf("Expected %d oks, got %d - %v", len(tC.wantOks), len(oks), oks)
			}

			if len(errs) != len(tC.wantErrs) {
				t.Errorf("Expected %d errors, got %d - %v", len(tC.wantErrs), len(errs), errs)
			}

			for _, ok := range oks {
				found := false
				for _, wantOk := range tC.wantOks {
					if ok == wantOk {
						found = true

						break
					}
				}

				if !found {
					t.Errorf("Unexpected ok: %s", ok)
				}
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
