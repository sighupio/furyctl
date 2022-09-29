// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"github.com/sighupio/furyctl/internal/execx"
)

type Tool interface {
	SrcPath() string
	Rename(basePath string) error
	CheckBinVersion(basePath string) error
	SupportsDownload() bool
	SetExecutor(executor execx.Executor)
}

func NewFactory() *Factory {
	return &Factory{}
}

type Factory struct{}

func (f *Factory) Create(name, version string) Tool {
	if name == "ansible" {
		return NewAnsible(version)
	}
	if name == "furyagent" {
		return NewFuryagent(version)
	}
	if name == "kubectl" {
		return NewKubectl(version)
	}
	if name == "kustomize" {
		return NewKustomize(version)
	}
	if name == "terraform" {
		return NewTerraform(version)
	}
	return nil
}
