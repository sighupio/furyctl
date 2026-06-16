// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var ErrAnsibleNotInstalled = errors.New("ansible is not installed, run 'furyctl download dependencies'")

// ansibleBundlePaths builds the ansible.Paths for a given workDir. When the kfd pins an
// ansible (bundle) version, the paths point at the self-contained bundle in
// binPath/ansible/<version>/ and are invoked through the bundle's Python; otherwise they fall
// back to the system "ansible"/"ansible-playbook" on PATH.
func ansibleBundlePaths(binPath, version, workDir string) ansible.Paths {
	if version == "" {
		return ansible.Paths{
			Ansible:         "ansible",
			AnsiblePlaybook: "ansible-playbook",
			WorkDir:         workDir,
		}
	}

	base := filepath.Join(binPath, "ansible", version)

	return ansible.Paths{
		Python:          filepath.Join(base, "python", "bin", "python3"),
		Ansible:         filepath.Join(base, "python", "bin", "ansible"),
		AnsiblePlaybook: filepath.Join(base, "python", "bin", "ansible-playbook"),
		CollectionsPath: filepath.Join(base, "collections"),
		WorkDir:         workDir,
	}
}

type ExtraToolsValidator struct {
	executor execx.Executor
	kfd      config.KFD
	binPath  string
}

func (x *ExtraToolsValidator) Validate(_ string) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	if err := x.validateAnsible(); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "ansible")
	}

	return oks, errs
}

func (x *ExtraToolsValidator) validateAnsible() error {
	// With a pinned ansible version this validates the downloaded bundle; otherwise it checks
	// the system ansible (legacy behavior).
	ansibleRunner := ansible.NewRunner(
		x.executor,
		ansibleBundlePaths(x.binPath, x.kfd.Tools.Common.Ansible.Version, ""),
	)

	if _, err := ansibleRunner.Version(); err != nil {
		return ErrAnsibleNotInstalled
	}

	return nil
}

func NewExtraToolsValidator(executor execx.Executor, kfd config.KFD, binPath string) *ExtraToolsValidator {
	return &ExtraToolsValidator{
		executor: executor,
		kfd:      kfd,
		binPath:  binPath,
	}
}
