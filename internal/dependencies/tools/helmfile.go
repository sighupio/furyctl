// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/helmfile"
)

type Helmfile struct {
	checker *checker
	version string
}

func NewHelmfile(runner *helmfile.Runner, version string) *Helmfile {
	return &Helmfile{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`.*`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimLeft(tokens[0], "v")
			},
			splitFn: func(version string) []string {
				return []string{version}
			},
		},
	}
}

func (h *Helmfile) CheckBinVersion() error {
	if err := h.checker.version(h.version); err != nil {
		return fmt.Errorf("helmfile: %w", err)
	}

	return nil
}
