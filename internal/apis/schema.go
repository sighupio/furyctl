// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import (
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises"
	"github.com/sighupio/furyctl/internal/distribution"
)

type ExtraSchemaValidator interface {
	Validate(confPath string) error
}

// PKIValidator checks the local PKI folder that a configuration points at.
type PKIValidator interface {
	ValidatePKI(confPath string) error
}

// NewPKIValidatorFactory returns the PKI validator of the kind, or nil for a kind that reads no local
// PKI. This check is separate from ExtraSchemaValidator because only `furyctl apply` and
// `furyctl validate config` run it. The other commands must not run it. For example,
// `furyctl create config` writes a configuration whose PKI folder does not exist yet.
func NewPKIValidatorFactory(apiVersion, kind string) PKIValidator {
	switch apiVersion {
	case distribution.APIVersionV1Alpha2:
		switch kind {
		case distribution.OnPremisesKind:
			return &onpremises.PKIValidator{}

		case distribution.ImmutableKind:
			return &immutable.PKIValidator{}

		default:
			return nil
		}

	default:
		return nil
	}
}

func NewExtraSchemaValidatorFactory(apiVersion, kind string) ExtraSchemaValidator {
	switch apiVersion {
	case distribution.APIVersionV1Alpha2:
		switch kind {
		case distribution.EKSClusterKind:
			return &ekscluster.ExtraSchemaValidator{}

		case distribution.KFDDistributionKind:
			return &kfddistribution.ExtraSchemaValidator{}

		case distribution.OnPremisesKind:
			return &onpremises.ExtraSchemaValidator{}

		case distribution.ImmutableKind:
			return &immutable.ExtraSchemaValidator{}

		default:
			return nil
		}

	default:
		return nil
	}
}
