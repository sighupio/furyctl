// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"errors"
	"fmt"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrInvalidNodePoolSize = errors.New("invalid node pool size")
	ErrVPNBucketNamePrefix = errors.New("vpn bucketNamePrefix is required when cluster name is longer than 19 characters")         //nolint: lll // long line
	ErrIamNameOverride     = errors.New("vpn iamResourcesNameOverride is required when cluster name is longer than 19 characters") //nolint: lll // long line
)

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

	//nolint:revive // ignore magic number linters
	if len(furyctlConf.Metadata.Name) > 19 && furyctlConf.Spec.Infrastructure != nil &&
		furyctlConf.Spec.Infrastructure.Vpn != nil &&
		(furyctlConf.Spec.Infrastructure.Vpn.Instances == nil ||
			(furyctlConf.Spec.Infrastructure.Vpn.Instances != nil &&
				*furyctlConf.Spec.Infrastructure.Vpn.Instances > 0)) {
		if furyctlConf.Spec.Infrastructure.Vpn.IamResourcesNameOverride == nil ||
			(furyctlConf.Spec.Infrastructure.Vpn.IamResourcesNameOverride != nil &&
				*furyctlConf.Spec.Infrastructure.Vpn.IamResourcesNameOverride == "") {
			return ErrIamNameOverride
		}

		if furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix == nil ||
			(furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix != nil &&
				*furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix == "") {
			return ErrVPNBucketNamePrefix
		}
	}

	return nil
}
