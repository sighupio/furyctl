// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/execx"
)

const ansibleVersionRegexString = "ansible \\[.*]"

var ansibleVersionRegex = regexp.MustCompile(ansibleVersionRegexString)

func NewAnsible(version string) *Ansible {
	return &Ansible{
		executor: execx.NewStdExecutor(),
		version:  version,
		os:       runtime.GOOS,
		arch:     "amd64",
	}
}

type Ansible struct {
	executor execx.Executor
	version  string
	os       string
	arch     string
}

func (a *Ansible) SetExecutor(executor execx.Executor) {
	a.executor = executor
}

func (a *Ansible) SupportsDownload() bool {
	return false
}

func (a *Ansible) SrcPath() string {
	return ""
}

func (a *Ansible) Rename(basePath string) error {
	return nil
}

//nolint:dupl // it will be refactored
func (a *Ansible) CheckBinVersion(binPath string) error {
	if a.version == "" {
		return fmt.Errorf("ansible: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "ansible")
	out, err := a.executor.Command(path, "--version").Output()
	if err != nil {
		return fmt.Errorf("error running %s: %w", path, err)
	}

	s := string(out)

	versionStringIndex := ansibleVersionRegex.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get ansible version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get ansible version from system")
	}

	systemAnsibleVersion := strings.TrimRight(versionStringTokens[len(versionStringTokens)-1], "]")

	if systemAnsibleVersion != a.version {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemAnsibleVersion, a.version)
	}

	return nil
}
