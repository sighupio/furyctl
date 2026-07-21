// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/kustomize"
)

type Kustomize struct {
	checker *checker
	version string
}

func NewKustomize(runner *kustomize.Runner, version string) *Kustomize {
	return &Kustomize{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`v(\S+)`),
			runner: runner,
			trimFn: func(tokens []string) string {
				return strings.TrimLeft(tokens[len(tokens)-1], "v")
			},
			splitFn: func(version string) []string {
				return strings.Split(version, "/")
			},
		},
	}
}

func (k *Kustomize) CheckBinVersion() error {
	if err := k.checker.version(k.version); err != nil {
		return fmt.Errorf("kustomize: %w", err)
	}

	return nil
}
