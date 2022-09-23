// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"runtime"

	"github.com/sighupio/furyctl/internal/semver"
)

func NewKubectl(version string) *Kubectl {
	return &Kubectl{
		version: version,
		os:      runtime.GOOS,
		arch:    "amd64",
	}
}

type Kubectl struct {
	version string
	os      string
	arch    string
}

func (k *Kubectl) SrcPath() string {
	return fmt.Sprintf(
		"https://dl.k8s.io/release/%s/bin/%s/%s/kubectl",
		semver.EnsurePrefix(k.version, "v"),
		k.os,
		k.arch,
	)
}

func (f *Kubectl) Rename(basePath string) error {
	return nil
}
