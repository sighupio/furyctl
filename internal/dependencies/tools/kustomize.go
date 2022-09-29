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

const kustomizeVersionRegexString = "kustomize/v(\\S*)"

var kustomizeVersionRegex = regexp.MustCompile(kustomizeVersionRegexString)

func NewKustomize(version string) *Kustomize {
	return &Kustomize{
		executor: execx.NewStdExecutor(),
		version:  version,
		os:       runtime.GOOS,
		arch:     "amd64",
	}
}

type Kustomize struct {
	executor execx.Executor
	version  string
	os       string
	arch     string
}

func (k *Kustomize) SetExecutor(executor execx.Executor) {
	k.executor = executor
}

func (k *Kustomize) SupportsDownload() bool {
	return true
}

func (k *Kustomize) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/%s/kustomize_%s_%s_%s.tar.gz",
		semver.EnsurePrefix(k.version, "v"),
		semver.EnsurePrefix(k.version, "v"),
		k.os,
		k.arch,
	)
}

func (k *Kustomize) Rename(basePath string) error {
	return nil
}

func (k *Kustomize) CheckBinVersion(binPath string) error {
	if k.version == "" {
		return fmt.Errorf("kustomize: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "kustomize")
	out, err := k.executor.Command(path, "version", "--short").Output()
	if err != nil {
		return fmt.Errorf("error running %s: %w", path, err)
	}

	s := string(out)

	versionStringIndex := kustomizeVersionRegex.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get kustomize version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, "/")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get kustomize version from system")
	}

	systemKustomizeVersion := strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "v")

	if systemKustomizeVersion != k.version {
		return fmt.Errorf("kustomize: %w - installed = %s, expected = %s", ErrWrongToolVersion, systemKustomizeVersion, k.version)
	}

	return nil
}
