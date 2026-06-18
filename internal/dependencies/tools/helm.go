// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/helm"
)

func NewHelm(runner *helm.Runner, version string) *Helm {
	return &Helm{
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
	checker *checker
	version string
}

func (h *Helm) CheckBinVersion() error {
	if err := h.checker.version(h.version); err != nil {
		return fmt.Errorf("helm: %w", err)
	}

	return nil
}
