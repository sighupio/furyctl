// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"fmt"
	"runtime"

	"github.com/sighupio/furyctl/internal/semver"
)

func NewTerraform(version string) *Terraform {
	return &Terraform{
		version: version,
		os:      runtime.GOOS,
		arch:    "amd64",
	}
}

type Terraform struct {
	version string
	os      string
	arch    string
}

func (k *Terraform) SrcPath() string {
	return fmt.Sprintf(
		"https://releases.hashicorp.com/terraform/%s/terraform_%s_%s_%s.zip",
		semver.EnsureNoPrefix(k.version, "v"),
		semver.EnsureNoPrefix(k.version, "v"),
		k.os,
		k.arch,
	)
}

func (f *Terraform) Rename(basePath string) error {
	return nil
}
