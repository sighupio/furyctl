// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/sed"
)

type Sed struct {
	checker *checker
	version string
}

func NewSed(runner *sed.Runner, version string) *Sed {
	return &Sed{
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

func (s *Sed) CheckBinVersion() error {
	if err := s.checker.version(s.version); err != nil {
		return fmt.Errorf("sed: %w", err)
	}

	return nil
}
