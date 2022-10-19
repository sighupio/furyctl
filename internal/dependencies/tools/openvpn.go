// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/openvpn"
)

func NewOpenvpn(runner *openvpn.Runner, version string) *Openvpn {
	return &Openvpn{
		arch:    "amd64",
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile(`^OpenVPN\\ (\\d+.\\d+.\\d+)`),
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
	arch    string
	checker *checker
	os      string
	version string
}

func (o *Openvpn) SupportsDownload() bool {
	return false
}

func (o *Openvpn) SrcPath() string {
	return ""
}

func (o *Openvpn) Rename(basePath string) error {
	return nil
}

func (o *Openvpn) CheckBinVersion() error {
	if err := o.checker.version(o.version); err != nil {
		return fmt.Errorf("openvpn: %w", err)
	}

	return nil
}
