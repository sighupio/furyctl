// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import (
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type ExtraToolsValidator interface {
	Validate(confPath string) ([]string, []error)
}

func NewExtraToolsValidatorFactory(executor execx.Executor, apiVersion, kind string) ExtraToolsValidator {
	switch apiVersion {
	case "kfd.sighup.io/v1alpha2":
		switch kind {
		case "EKSCluster":
			return eks.NewExtraToolsValidator(executor)

		default:
			return nil
		}

	default:
		return nil
	}
}
