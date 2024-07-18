// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"errors"
	"fmt"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	ErrInvalidNodePoolSize                  = errors.New("invalid node pool size")
	ErrVPNBucketNamePrefix                  = errors.New(".spec.infrastructure.vpn.bucketNamePrefix is required when cluster name is longer than 19 characters")                                   //nolint: lll // long line
	ErrIAMUserNameOverride                  = errors.New(".spec.infrastructure.vpn.iamUserNameOverride is required when cluster name is longer than 19 characters")                                //nolint: lll // long line
	ErrClusterIAMRoleNamePrefixOverride     = errors.New(".spec.kubernetes.clusterIAMRoleNamePrefixOverride is required when cluster name is longer than 40 characters")                           //nolint: lll // long line
	ErrWorkersIAMRoleNamePrefixOverride     = errors.New(".spec.kubernetes.workersIAMRoleNamePrefixOverride is required when cluster name is longer than 40 characters")                           //nolint: lll // long line
	ErrEBSCSIDriverIAMRoleNameOverride      = errors.New(".spec.distribution.modules.aws.ebsCsiDriver.overrides.iamRoleName is required when cluster name is longer than 40 characters")           //nolint: lll // long line
	ErrClusterAutoscalerIAMRoleNameOverride = errors.New(".spec.distribution.modules.aws.clusterAutoscaler.overrides.iamRoleName is required when cluster name is longer than 40 characters")      //nolint: lll // long line
	ErrLBControllerIAMRoleNameOverride      = errors.New(".spec.distribution.modules.aws.loadbalancerController.overrides.iamRoleName is required when cluster name is longer than 40 characters") //nolint: lll // long line
)

type ExtraSchemaValidator struct{}

func (v *ExtraSchemaValidator) Validate(confPath string) error {
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

	return v.validateClusterName(&furyctlConf)
}

func (v *ExtraSchemaValidator) validateClusterName(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	if err := v.validateInfraVPNOverrides(furyctlConf); err != nil {
		return err
	}

	if err := v.validateKubernetesOverrides(furyctlConf); err != nil {
		return err
	}

	return v.validateAWSModuleOverrides(furyctlConf)
}

func (*ExtraSchemaValidator) validateInfraVPNOverrides(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	//nolint:revive // ignore magic number linters
	if len(furyctlConf.Metadata.Name) > 19 && furyctlConf.Spec.Infrastructure != nil &&
		furyctlConf.Spec.Infrastructure.Vpn != nil &&
		(furyctlConf.Spec.Infrastructure.Vpn.Instances == nil ||
			(furyctlConf.Spec.Infrastructure.Vpn.Instances != nil &&
				*furyctlConf.Spec.Infrastructure.Vpn.Instances > 0)) {
		if furyctlConf.Spec.Infrastructure.Vpn.IamUserNameOverride == nil ||
			(furyctlConf.Spec.Infrastructure.Vpn.IamUserNameOverride != nil &&
				*furyctlConf.Spec.Infrastructure.Vpn.IamUserNameOverride == "") {
			return ErrIAMUserNameOverride
		}

		if furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix == nil ||
			(furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix != nil &&
				*furyctlConf.Spec.Infrastructure.Vpn.BucketNamePrefix == "") {
			return ErrVPNBucketNamePrefix
		}
	}

	return nil
}

func (*ExtraSchemaValidator) validateKubernetesOverrides(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	//nolint:revive,mnd // ignore magic number linters
	if len(furyctlConf.Metadata.Name) <= 40 {
		return nil
	}

	if furyctlConf.Spec.Kubernetes.ClusterIAMRoleNamePrefixOverride == nil ||
		(furyctlConf.Spec.Kubernetes.ClusterIAMRoleNamePrefixOverride != nil &&
			*furyctlConf.Spec.Kubernetes.ClusterIAMRoleNamePrefixOverride == "") {
		return ErrClusterIAMRoleNamePrefixOverride
	}

	if furyctlConf.Spec.Kubernetes.WorkersIAMRoleNamePrefixOverride == nil ||
		(furyctlConf.Spec.Kubernetes.WorkersIAMRoleNamePrefixOverride != nil &&
			*furyctlConf.Spec.Kubernetes.WorkersIAMRoleNamePrefixOverride == "") {
		return ErrWorkersIAMRoleNamePrefixOverride
	}

	return nil
}

func (v *ExtraSchemaValidator) validateAWSModuleOverrides(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	//nolint:revive,mnd // ignore magic number linters
	if len(furyctlConf.Metadata.Name) <= 40 {
		return nil
	}

	if err := v.validateEbsCsiDriverOverride(furyctlConf); err != nil {
		return err
	}

	if err := v.validateClusterAutoscalerOverride(furyctlConf); err != nil {
		return err
	}

	return v.validateLoadBalancerControllerOverride(furyctlConf)
}

//nolint:dupl // false positive
func (*ExtraSchemaValidator) validateEbsCsiDriverOverride(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	if furyctlConf.Spec.Distribution.Modules.Aws == nil ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides == nil) ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides.IamRoleName == nil) ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides.IamRoleName != nil &&
			*furyctlConf.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides.IamRoleName == "") {
		return ErrEBSCSIDriverIAMRoleNameOverride
	}

	return nil
}

//nolint:dupl // false positive
func (*ExtraSchemaValidator) validateClusterAutoscalerOverride(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	if furyctlConf.Spec.Distribution.Modules.Aws == nil ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.ClusterAutoscaler.Overrides == nil) ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.ClusterAutoscaler.Overrides != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.ClusterAutoscaler.Overrides.IamRoleName == nil) ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.ClusterAutoscaler.Overrides != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.ClusterAutoscaler.Overrides.IamRoleName != nil &&
			*furyctlConf.Spec.Distribution.Modules.Aws.ClusterAutoscaler.Overrides.IamRoleName == "") {
		return ErrClusterAutoscalerIAMRoleNameOverride
	}

	return nil
}

//nolint:dupl // false positive
func (*ExtraSchemaValidator) validateLoadBalancerControllerOverride(furyctlConf *private.EksclusterKfdV1Alpha2) error {
	if furyctlConf.Spec.Distribution.Modules.Aws == nil ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.LoadBalancerController.Overrides == nil) ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.LoadBalancerController.Overrides != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.LoadBalancerController.Overrides.IamRoleName == nil) ||
		(furyctlConf.Spec.Distribution.Modules.Aws != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.LoadBalancerController.Overrides != nil &&
			furyctlConf.Spec.Distribution.Modules.Aws.LoadBalancerController.Overrides.IamRoleName != nil &&
			*furyctlConf.Spec.Distribution.Modules.Aws.LoadBalancerController.Overrides.IamRoleName == "") {
		return ErrLBControllerIAMRoleNameOverride
	}

	return nil
}
