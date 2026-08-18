// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import (
	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises"
	"github.com/sighupio/furyctl/internal/distribution"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type ExtraToolsValidator interface {
	Validate(confPath string) ([]string, []error)
}

func NewExtraToolsValidatorFactory(
	executor execx.Executor,
	apiVersion,
	kind string,
	kfdManifest config.KFD,
	binPath string,
) ExtraToolsValidator {
	switch apiVersion {
	case distribution.APIVersionV1Alpha2:
		switch kind {
		case distribution.EKSClusterKind:
			return ekscluster.NewExtraToolsValidator(executor)

		case distribution.OnPremisesKind:
			return onpremises.NewExtraToolsValidator(executor, kfdManifest, binPath)

		default:
			return nil
		}

	default:
		return nil
	}
}
