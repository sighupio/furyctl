// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tool

import (
	"path/filepath"

	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

const (
	Ansible   = "ansible"
	Awscli    = "awscli"
	Furyagent = "furyagent"
	Kubectl   = "kubectl"
	Kustomize = "kustomize"
	Openvpn   = "openvpn"
	Terraform = "terraform"
)

type Runner interface {
	Version() (string, error)
	CmdPath() string
}

type RunnerFactoryPaths struct {
	Bin string
}

func NewRunnerFactory(executor execx.Executor, paths RunnerFactoryPaths) *RunnerFactory {
	return &RunnerFactory{
		executor: executor,
		paths:    paths,
	}
}

type RunnerFactory struct {
	executor execx.Executor
	paths    RunnerFactoryPaths
}

func (rf *RunnerFactory) Create(name, version, workDir string) Runner {
	if name == Ansible {
		return ansible.NewRunner(rf.executor, ansible.Paths{
			Ansible: name,
			WorkDir: workDir,
		})
	}

	if name == Awscli {
		return awscli.NewRunner(rf.executor, awscli.Paths{
			Awscli:  "aws",
			WorkDir: workDir,
		})
	}

	if name == Furyagent {
		return furyagent.NewRunner(rf.executor, furyagent.Paths{
			Furyagent: filepath.Join(rf.paths.Bin, name, version, name),
			WorkDir:   workDir,
		})
	}

	if name == Kubectl {
		return kubectl.NewRunner(rf.executor, kubectl.Paths{
			Kubectl: filepath.Join(rf.paths.Bin, name, version, name),
			WorkDir: workDir,
		},
			true, true)
	}

	if name == Kustomize {
		return kustomize.NewRunner(rf.executor, kustomize.Paths{
			Kustomize: filepath.Join(rf.paths.Bin, name, version, name),
			WorkDir:   workDir,
		})
	}

	if name == Openvpn {
		return openvpn.NewRunner(rf.executor, openvpn.Paths{
			Openvpn: filepath.Join(rf.paths.Bin, name, version, name),
			WorkDir: workDir,
		})
	}

	if name == Terraform {
		return terraform.NewRunner(rf.executor, terraform.Paths{
			Terraform: filepath.Join(rf.paths.Bin, name, version, name),
			WorkDir:   workDir,
		})
	}

	return nil
}
