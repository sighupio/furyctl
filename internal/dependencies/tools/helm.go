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

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/helm"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

func NewHelm(runner *helm.Runner, version string) *Helm {
	return &Helm{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`.*`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimLeft(tokens[0], "v")
			},
			splitFn: func(version string) []string {
				return strings.Split(version, "+")
			},
		},
	}
}

type Helm struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Helm) SupportsDownload() bool {
	return true
}

func (h *Helm) SrcPath() string {
	return fmt.Sprintf(
		"https://get.helm.sh/helm-%s-%s-%s.tar.gz",
		semver.EnsurePrefix(h.version),
		h.os,
		h.arch,
	)
}

func (h *Helm) Rename(basePath string) error {
	oldPath := filepath.Join(basePath, fmt.Sprintf("%s-%s/helm", h.os, h.arch))
	newPath := filepath.Join(basePath, "helm")

	if err := iox.CopyFile(oldPath, newPath); err != nil {
		return fmt.Errorf("error while renaming helm: %w", err)
	}

	// if err := os.RemoveAll(filepath.Join(basePath, fmt.Sprintf("%s-%s", h.os, h.arch))); err != nil {
	// 	return fmt.Errorf("error while renaming helm: %w", err)
	// }

	return nil
}

func (h *Helm) CheckBinVersion() error {
	if err := h.checker.version(h.version); err != nil {
		return fmt.Errorf("helm: %w", err)
	}

	return nil
}
