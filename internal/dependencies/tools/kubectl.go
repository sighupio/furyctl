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
	"github.com/sighupio/furyctl/internal/semver"
)

const furyctlVersionRegexpString = "GitVersion:\"([^\"]*)\""

var furyctlVersionRegexp = regexp.MustCompile(furyctlVersionRegexpString)

func NewKubectl(version string) *Kubectl {
	return &Kubectl{
		executor: execx.NewStdExecutor(),
		version:  version,
		os:       runtime.GOOS,
		arch:     "amd64",
	}
}

type Kubectl struct {
	executor execx.Executor
	version  string
	os       string
	arch     string
}

func (k *Kubectl) SetExecutor(executor execx.Executor) {
	k.executor = executor
}

func (k *Kubectl) SupportsDownload() bool {
	return true
}

func (k *Kubectl) SrcPath() string {
	return fmt.Sprintf(
		"https://dl.k8s.io/release/%s/bin/%s/%s/kubectl",
		semver.EnsurePrefix(k.version, "v"),
		k.os,
		k.arch,
	)
}

func (k *Kubectl) Rename(basePath string) error {
	return nil
}

func (k *Kubectl) CheckBinVersion(binPath string) error {
	if k.version == "" {
		return fmt.Errorf("kubectl: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "kubectl")
	out, err := k.executor.Command(path, "version", "--client").Output()
	if err != nil {
		return fmt.Errorf("error running %s: %w", path, err)
	}

	s := string(out)

	versionStringIndex := furyctlVersionRegexp.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get kubectl version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, ":")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get kubectl version from system")
	}

	systemKubectlVersion := strings.TrimRight(
		strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "\"v"),
		"\"",
	)

	if systemKubectlVersion != k.version {
		return fmt.Errorf("%w: installed = %s, expected = %s", ErrWrongToolVersion, systemKubectlVersion, k.version)
	}

	return nil
}
