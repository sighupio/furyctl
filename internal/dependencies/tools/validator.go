// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"errors"
	"reflect"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
	itool "github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	ErrEmptyToolVersion = errors.New("empty tool version")
	ErrWrongToolVersion = errors.New("wrong tool version")
)

func NewValidator(executor execx.Executor, binPath string) *Validator {
	return &Validator{
		executor: executor,
		toolFactory: NewFactory(executor, FactoryPaths{
			Bin: binPath,
		}),
	}
}

type Validator struct {
	executor    execx.Executor
	toolFactory *Factory
}

func (tv *Validator) Validate(kfdManifest config.KFD, tfState config.State) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	tls := reflect.ValueOf(kfdManifest.Tools)
	for i := 0; i < tls.NumField(); i++ {
		for j := 0; j < tls.Field(i).NumField(); j++ {
			if version, ok := tls.Field(i).Field(j).Interface().(config.Tool); ok {
				if version.String() == "" {
					continue
				}

				name := strings.ToLower(tls.Field(i).Type().Field(j).Name)

				tool := tv.toolFactory.Create(name, version.String())
				if err := tool.CheckBinVersion(); err != nil {
					errs = append(errs, err)
				} else {
					oks = append(oks, name)
				}
			}
		}
	}

	if tfState.S3.BucketName != "" {
		tool := tv.toolFactory.Create(itool.Awscli, "*")
		if err := tool.CheckBinVersion(); err != nil {
			errs = append(errs, err)
		} else {
			oks = append(oks, "aws")
		}
	}

	return oks, errs
}
