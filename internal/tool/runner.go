// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tool

import (
	"path/filepath"

	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/git"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/kustomize"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Name string

const (
	Ansible   Name = "ansible"
	Awscli    Name = "awscli"
	Furyagent Name = "furyagent"
	Git       Name = "git"
	Kubectl   Name = "kubectl"
	Kustomize Name = "kustomize"
	Openvpn   Name = "openvpn"
	Terraform Name = "terraform"
)

type Runner interface {
	Version() (string, error)
	CmdPath() string
	Stop() error
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

func (rf *RunnerFactory) Create(name Name, version, workDir string) Runner {
	switch name {
	case Ansible:
		return ansible.NewRunner(rf.executor, ansible.Paths{
			Ansible: string(name),
			WorkDir: workDir,
		})

	case Awscli:
		return awscli.NewRunner(rf.executor, awscli.Paths{
			Awscli:  "aws",
			WorkDir: workDir,
		})

	case Furyagent:
		return furyagent.NewRunner(rf.executor, furyagent.Paths{
			Furyagent: filepath.Join(rf.paths.Bin, string(name), version, string(name)),
			WorkDir:   workDir,
		})

	case Git:
		return git.NewRunner(rf.executor, git.Paths{
			Git:     "git",
			WorkDir: workDir,
		})

	case Kubectl:
		return kubectl.NewRunner(
			rf.executor,
			kubectl.Paths{
				Kubectl: filepath.Join(rf.paths.Bin, string(name), version, string(name)),
				WorkDir: workDir,
			},
			false, true, true,
		)

	case Kustomize:
		return kustomize.NewRunner(rf.executor, kustomize.Paths{
			Kustomize: filepath.Join(rf.paths.Bin, string(name), version, string(name)),
			WorkDir:   workDir,
		})

	case Openvpn:
		return openvpn.NewRunner(rf.executor, openvpn.Paths{
			Openvpn: filepath.Join(rf.paths.Bin, string(name), version, string(name)),
			WorkDir: workDir,
		})

	case Terraform:
		return terraform.NewRunner(rf.executor, terraform.Paths{
			Terraform: filepath.Join(rf.paths.Bin, string(name), version, string(name)),
			WorkDir:   workDir,
		})

	default:
		return nil
	}
}
