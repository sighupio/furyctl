// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var ErrAnsibleNotInstalled = errors.New("ansible is not installed, run 'furyctl download dependencies'")

type ExtraToolsValidator struct {
	executor execx.Executor
	kfd      config.KFD
	binPath  string
}

func NewExtraToolsValidator(executor execx.Executor, kfd config.KFD, binPath string) *ExtraToolsValidator {
	return &ExtraToolsValidator{
		executor: executor,
		kfd:      kfd,
		binPath:  binPath,
	}
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
	// With a pinned ansible version this validates the mise-managed ansible; otherwise it checks the
	// system ansible (legacy behaviour).
	ansibleRunner := ansible.NewRunner(
		x.executor,
		ansible.PathsForVersion(x.binPath, x.kfd.Tools.OnPremises.Ansible.Version, ""),
	)

	if _, err := ansibleRunner.Version(); err != nil {
		return ErrAnsibleNotInstalled
	}

	return nil
}
