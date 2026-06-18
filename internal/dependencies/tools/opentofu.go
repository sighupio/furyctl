// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/terraform"
)

func NewOpenTofu(runner *terraform.Runner, version string) *OpenTofu {
	return &OpenTofu{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("OpenTofu .*"),
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

type OpenTofu struct {
	checker *checker
	version string
}

func (t *OpenTofu) CheckBinVersion() error {
	if err := t.checker.version(t.version); err != nil {
		return fmt.Errorf("opentofu: %w", err)
	}

	return nil
}
