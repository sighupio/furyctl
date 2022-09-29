// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"errors"
	"reflect"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/execx"
)

var (
	ErrEmptyToolVersion = errors.New("empty tool version")
	ErrWrongToolVersion = errors.New("wrong tool version")
)

func NewValidator(executor execx.Executor) *Validator {
	return &Validator{
		executor: executor,
	}
}

type Validator struct {
	executor    execx.Executor
	toolFactory Factory
}

func (tv *Validator) Validate(kfdManifest config.KFD, binPath string) []error {
	var errs []error

	tls := reflect.ValueOf(kfdManifest.Tools)
	for i := 0; i < tls.NumField(); i++ {
		var err error

		name := strings.ToLower(tls.Type().Field(i).Name)
		version := tls.Field(i).Interface().(string)

		if version == "" {
			continue
		}

		tool := tv.toolFactory.Create(name, version)
		tool.SetExecutor(tv.executor)

		if tool.SupportsDownload() {
			err = tool.CheckBinVersion(binPath)
		} else {
			err = tool.CheckBinVersion("")
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
