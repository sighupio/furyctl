// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package tools_test

import (
	"errors"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/apis/config"
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
			desc: "all common tools are installed in their correct version",
			manifest: config.KFD{
				Version: "1.29.0",
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.21.1"},
						Kustomize: config.KFDTool{Version: "3.9.4"},
						Yq:        config.KFDTool{Version: "4.34.1"},
						Helm:      config.KFDTool{Version: "3.12.3"},
						Helmfile:  config.KFDTool{Version: "0.156.0"},
						Kapp:      config.KFDTool{Version: "0.62.0"},
					},
				},
			},
			state: config.Furyctl{
				Spec: config.FuryctlSpec{},
			},
			wantOks: []string{
				"kubectl",
				"kustomize",
				"yq",
				"helm",
				"helmfile",
				"kapp",
			},
		},
		{
			desc: "all common tools are installed in their wrong version",
			manifest: config.KFD{
				Version: "1.29.0",
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.22.0"},
						Kustomize: config.KFDTool{Version: "3.5.3"},
						Yq:        config.KFDTool{Version: "4.33.0"},
						Helm:      config.KFDTool{Version: "3.11.3"},
						Helmfile:  config.KFDTool{Version: "0.155.0"},
						Kapp:      config.KFDTool{Version: "0.61.0"},
					},
				},
			},
			state: config.Furyctl{
				Spec: config.FuryctlSpec{},
			},
			wantErrs: []error{
				errors.New("kubectl: wrong tool version - installed = 1.21.1, expected = 1.22.0"),
				errors.New("kustomize: wrong tool version - installed = 3.9.4, expected = 3.5.3"),
				errors.New("yq: wrong tool version - installed = 4.34.1, expected = 4.33.0"),
				errors.New("helm: wrong tool version - installed = 3.12.3, expected = 3.11.3"),
				errors.New("helmfile: wrong tool version - installed = 0.156.0, expected = 0.155.0"),
				errors.New("kapp: wrong tool version - installed = 0.62.0, expected = 0.61.0"),
			},
		},
		{
			desc: "all tools for EKSCluster kind are installed (eks tools include opentofu and furyagent)",
			manifest: config.KFD{
				Version: "1.29.0",
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.21.1"},
						Kustomize: config.KFDTool{Version: "3.9.4"},
						Yq:        config.KFDTool{Version: "4.34.1"},
						Helm:      config.KFDTool{Version: "3.12.3"},
						Helmfile:  config.KFDTool{Version: "0.156.0"},
						Kapp:      config.KFDTool{Version: "0.62.0"},
					},
					Eks: config.KFDToolsEks{
						Awscli:    config.KFDTool{Version: "2.8.12"},
						OpenTofu:  config.KFDTool{Version: "1.10.0"},
						Furyagent: config.KFDTool{Version: "0.3.0"},
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
				"yq",
				"helm",
				"helmfile",
				"kapp",
				"awscli",
				"opentofu",
				"furyagent",
				"openvpn",
			},
		},
		{
			desc: "EKSCluster with legacy tools under common (backward compat, distros < 1.34.2)",
			manifest: config.KFD{
				Version: "1.33.2",
				Tools: config.KFDTools{
					Common: config.KFDToolsCommon{
						Kubectl:   config.KFDTool{Version: "1.21.1"},
						Kustomize: config.KFDTool{Version: "3.9.4"},
						Yq:        config.KFDTool{Version: "4.34.1"},
						Helm:      config.KFDTool{Version: "3.12.3"},
						Helmfile:  config.KFDTool{Version: "0.156.0"},
						Kapp:      config.KFDTool{Version: "0.62.0"},
						OpenTofu:  config.KFDTool{Version: "1.10.0"},
						Furyagent: config.KFDTool{Version: "0.3.0"},
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
				"yq",
				"helm",
				"helmfile",
				"kapp",
				"opentofu",
				"furyagent",
				"awscli",
				"openvpn",
			},
		},
	}
	for _, tC := range testCases {
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
				if !slices.Contains(tC.wantOks, ok) {
					t.Errorf("Unexpected ok: %s", ok)
				}
			}

			for _, err := range errs {
				if !slices.ContainsFunc(tC.wantErrs, func(wantErr error) bool {
					return strings.Trim(err.Error(), "\n") == strings.Trim(wantErr.Error(), "\n")
				}) {
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
				"sed",
			},
		},
	}

	for _, tC := range testCases {
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
				if !slices.Contains(tC.wantOks, ok) {
					t.Errorf("Unexpected ok: %s", ok)
				}
			}

			for _, err := range errs {
				if !slices.ContainsFunc(tC.wantErrs, func(wantErr error) bool {
					return strings.Trim(err.Error(), "\n") == strings.Trim(wantErr.Error(), "\n")
				}) {
					t.Errorf("Unexpected error: %s", err)
				}
			}
		})
	}
}
