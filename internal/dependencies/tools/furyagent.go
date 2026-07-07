// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/furyagent"
)

func NewFuryagent(runner *furyagent.Runner, version string) *Furyagent {
	return &Furyagent{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`version (\S*)`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return tokens[len(tokens)-1]
			},
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
		},
	}
}

type Furyagent struct {
	checker *checker
	version string
}

func (f *Furyagent) CheckBinVersion() error {
	if err := f.checker.version(f.version); err != nil {
		return fmt.Errorf("furyagent: %w", err)
	}

	return nil
}
