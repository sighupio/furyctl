// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/kubectl"
)

type Kubectl struct {
	checker *checker
	version string
}

func NewKubectl(runner *kubectl.Runner, version string) *Kubectl {
	return &Kubectl{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("\"gitVersion\": \"v([^\"]*)\""),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimRight(
					strings.TrimLeft(tokens[len(tokens)-1], " \"v"),
					"\"",
				)
			},
			splitFn: func(version string) []string {
				return strings.Split(version, ":")
			},
		},
	}
}

func (k *Kubectl) CheckBinVersion() error {
	if err := k.checker.version(k.version); err != nil {
		return fmt.Errorf("kubectl: %w", err)
	}

	return nil
}
