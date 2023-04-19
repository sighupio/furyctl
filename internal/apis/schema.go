// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apis

import "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks"

type ExtraSchemaValidator interface {
	Validate(confPath string) error
}

func NewExtraSchemaValidatorFactory(apiVersion, kind string) ExtraSchemaValidator {
	switch apiVersion {
	case "kfd.sighup.io/v1alpha2":
		switch kind {
		case "EKSCluster":
			return &eks.ExtraSchemaValidator{}

		default:
			return nil
		}

	default:
		return nil
	}
}
