// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"fmt"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrInvalidNodePoolSize = fmt.Errorf("invalid node pool size")

type ExtraSchemaValidator struct{}

func (*ExtraSchemaValidator) Validate(confPath string) error {
	furyctlConf, err := yamlx.FromFileV3[private.EksclusterKfdV1Alpha2](confPath)
	if err != nil {
		return err
	}

	for i, nodePool := range furyctlConf.Spec.Kubernetes.NodePools {
		if nodePool.Size.Max < nodePool.Size.Min {
			return fmt.Errorf(
				"%w: element %d's max size(%d) must be greater than or equal to its min(%d)",
				ErrInvalidNodePoolSize,
				i,
				nodePool.Size.Max,
				nodePool.Size.Min,
			)
		}
	}

	return nil
}
