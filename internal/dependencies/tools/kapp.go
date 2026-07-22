// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/kapp"
)

type Kapp struct {
	checker *checker
	version string
}

func NewKapp(runner *kapp.Runner, version string) *Kapp {
	return &Kapp{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("kapp version " + semver.Regex),
			runner: runner,
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
			trimFn: func(tokens []string) string {
				return tokens[len(tokens)-1]
			},
		},
	}
}

func (k *Kapp) CheckBinVersion() error {
	if err := k.checker.version(k.version); err != nil {
		return fmt.Errorf("kapp: %w", err)
	}

	return nil
}
