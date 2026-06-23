// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package private_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
)

// TestAntiDrift decodes a furyctl.yaml exercising the fields furyctl reads from
// an EKSCluster config and asserts they decode (guards yaml-tag drift from the
// distribution schema). The injection-target fields (IAM ARNs, VPC id) are
// written by furyctl, not decoded, so they are not exercised here.
func TestAntiDrift(t *testing.T) {
	t.Parallel()

	const sample = `
kind: EKSCluster
metadata:
  name: my-cluster
spec:
  region: eu-west-1
  infrastructure:
    vpc:
      network:
        cidr: 10.0.0.0/16
    vpn:
      instances: 2
      port: 1194
      bucketNamePrefix: pfx
      operatorName: op
  kubernetes:
    apiServer:
      privateAccess: true
      publicAccess: false
    vpcId: vpc-123
    nodePools:
      - size:
          min: 1
          max: 3
  distribution:
    modules:
      dr:
        type: eks
      aws:
        ebsCsiDriver:
          overrides:
            iamRoleName: role
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: bkt
          keyPrefix: pfx
          region: eu-west-1
`

	var c private.EksclusterKfdV1Alpha2
	if err := yaml.Unmarshal([]byte(sample), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if c.Kind != "EKSCluster" || c.Metadata.Name != "my-cluster" || c.Spec.Region != "eu-west-1" {
		t.Errorf("top-level fields did not decode: kind=%q name=%q region=%q", c.Kind, c.Metadata.Name, c.Spec.Region)
	}

	if c.Spec.Infrastructure == nil || c.Spec.Infrastructure.Vpc == nil ||
		c.Spec.Infrastructure.Vpc.Network.Cidr != "10.0.0.0/16" {
		t.Errorf("Infrastructure.Vpc.Network.Cidr did not decode")
	}

	if c.Spec.Infrastructure.Vpn == nil || c.Spec.Infrastructure.Vpn.Instances == nil ||
		*c.Spec.Infrastructure.Vpn.Instances != 2 || c.Spec.Infrastructure.Vpn.Port == nil {
		t.Errorf("Infrastructure.Vpn fields did not decode")
	}

	if !c.Spec.Kubernetes.ApiServer.PrivateAccess || c.Spec.Kubernetes.ApiServer.PublicAccess {
		t.Errorf("Kubernetes.ApiServer access flags did not decode")
	}

	if c.Spec.Kubernetes.VpcId == nil || *c.Spec.Kubernetes.VpcId != "vpc-123" {
		t.Errorf("Kubernetes.VpcId did not decode")
	}

	if len(c.Spec.Kubernetes.NodePools) != 1 ||
		c.Spec.Kubernetes.NodePools[0].Size.Min != 1 || c.Spec.Kubernetes.NodePools[0].Size.Max != 3 {
		t.Errorf("Kubernetes.NodePools[0].Size did not decode")
	}

	if c.Spec.Distribution.Modules.Dr.Type != "eks" {
		t.Errorf("Distribution.Modules.Dr.Type did not decode, got %q", c.Spec.Distribution.Modules.Dr.Type)
	}

	if c.Spec.Distribution.Modules.Aws == nil ||
		c.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides == nil ||
		c.Spec.Distribution.Modules.Aws.EbsCsiDriver.Overrides.IamRoleName == nil {
		t.Errorf("Aws.EbsCsiDriver.Overrides.IamRoleName did not decode")
	}

	if c.Spec.ToolsConfiguration.Terraform == nil ||
		c.Spec.ToolsConfiguration.Terraform.State.S3.BucketName != "bkt" {
		t.Errorf("ToolsConfiguration.Terraform.State.S3.BucketName did not decode")
	}
}
