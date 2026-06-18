// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/openvpn"
)

func NewOpenvpn(runner *openvpn.Runner, version string) *Openvpn {
	return &Openvpn{
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`^OpenVPN (\d+.\d+.\d+)`),
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

type Openvpn struct {
	checker *checker
	version string
}

func (o *Openvpn) CheckBinVersion() error {
	if err := o.checker.version(o.version); err != nil {
		return fmt.Errorf("openvpn: %w", err)
	}

	return nil
}
