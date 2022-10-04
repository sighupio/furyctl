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

func NewValidator(executor execx.Executor, binPath string) *Validator {
	return &Validator{
		executor: executor,
		toolFactory: NewFactory(execx.NewStdExecutor(), FactoryPaths{
			Bin: binPath,
		}),
	}
}

type Validator struct {
	executor    execx.Executor
	toolFactory *Factory
}

func (tv *Validator) Validate(kfdManifest config.KFD) []error {
	var errs []error

	tls := reflect.ValueOf(kfdManifest.Tools)
	for i := 0; i < tls.NumField(); i++ {
		name := strings.ToLower(tls.Type().Field(i).Name)
		version := tls.Field(i).Interface().(string)

		if version == "" {
			continue
		}

		tool := tv.toolFactory.Create(name, version)

		if err := tool.CheckBinVersion(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
