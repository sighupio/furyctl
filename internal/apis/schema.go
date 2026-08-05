// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import (
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises"
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
	case "kfd.sighup.io/v1alpha2":
		switch kind {
		case "OnPremises":
			return &onpremises.PKIValidator{}

		case "Immutable":
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
	case "kfd.sighup.io/v1alpha2":
		switch kind {
		case "EKSCluster":
			return &ekscluster.ExtraSchemaValidator{}

		case "KFDDistribution":
			return &kfddistribution.ExtraSchemaValidator{}

		case "OnPremises":
			return &onpremises.ExtraSchemaValidator{}

		case "Immutable":
			return &immutable.ExtraSchemaValidator{}

		default:
			return nil
		}

	default:
		return nil
	}
}
