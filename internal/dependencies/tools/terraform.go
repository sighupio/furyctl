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

type Terraform struct {
	checker *checker
	version string
}

func NewTerraform(runner *terraform.Runner, version string) *Terraform {
	return &Terraform{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("Terraform .*"),
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

func (t *Terraform) CheckBinVersion() error {
	if err := t.checker.version(t.version); err != nil {
		return fmt.Errorf("terraform: %w", err)
	}

	return nil
}
