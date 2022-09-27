// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/execx"
	"github.com/sighupio/furyctl/internal/semver"
)

const furyAgentVersionRegexString = "version (\\S*)"

var furyAgentVersionRegex = regexp.MustCompile(furyAgentVersionRegexString)

func NewFuryagent(version string) *Furyagent {
	return &Furyagent{
		executor: execx.NewStdExecutor(),
		version:  version,
		os:       runtime.GOOS,
		arch:     "amd64",
	}
}

type Furyagent struct {
	executor execx.Executor
	version  string
	os       string
	arch     string
}

func (f *Furyagent) SetExecutor(executor execx.Executor) {
	f.executor = executor
}

func (f *Furyagent) SupportsDownload() bool {
	return true
}

func (f *Furyagent) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/sighupio/furyagent/releases/download/%s/furyagent-%s-%s",
		semver.EnsurePrefix(f.version, "v"),
		f.os,
		f.arch,
	)
}

func (f *Furyagent) Rename(basePath string) error {
	oldName := fmt.Sprintf("furyagent-%s-%s", f.os, f.arch)
	newName := "furyagent"

	return os.Rename(filepath.Join(basePath, oldName), filepath.Join(basePath, newName))
}

func (f *Furyagent) CheckBinVersion(binPath string) error {
	if f.version == "" {
		return fmt.Errorf("furyagent: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "furyagent")
	out, err := f.executor.Command(path, "version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	versionStringIndex := furyAgentVersionRegex.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get furyagent version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get furyagent version from system")
	}

	systemFuryagentVersion := versionStringTokens[len(versionStringTokens)-1]

	if systemFuryagentVersion != f.version {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemFuryagentVersion, f.version)
	}

	return nil
}
