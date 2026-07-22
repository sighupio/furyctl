// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/yq"
)

type Yq struct {
	checker *checker
	version string
}

func NewYq(runner *yq.Runner, version string) *Yq {
	return &Yq{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`yq \(https:\/\/github\.com\/mikefarah\/yq\/\) version .*`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimLeft(tokens[len(tokens)-1], "v")
			},
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
		},
	}
}

func (y *Yq) CheckBinVersion() error {
	if err := y.checker.version(y.version); err != nil {
		return fmt.Errorf("yq: %w", err)
	}

	return nil
}
