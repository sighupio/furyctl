// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/tool/awscli"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type EKSInfra struct {
	VpcID     string
	SubnetIDs []string
}

type EKSInfraDeleter struct {
	awsRunner *awscli.Runner
	region    string
}

type EKSInfraCreator struct {
	awsRunner *awscli.Runner
	region    string
}

func NewEKSInfraDeleter(workDir, region string) *EKSInfraDeleter {
	return &EKSInfraDeleter{
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: workDir,
			},
		),
		region: region,
	}
}

func (id *EKSInfraDeleter) DeleteVpc(vpcID string) error {
	if _, err := id.awsRunner.Ec2(
		false,
		"delete-vpc",
		"--vpc-id",
		vpcID,
		"--region",
		id.region,
	); err != nil {
		return fmt.Errorf("error deleting vpc %s: %w", vpcID, err)
	}

	return nil
}

func (id *EKSInfraDeleter) DeleteSubnet(subnetID string) error {
	if _, err := id.awsRunner.Ec2(
		false,
		"delete-subnet",
		"--subnet-id",
		subnetID,
		"--region",
		id.region,
	); err != nil {
		return fmt.Errorf("error deleting subnet %s: %w", subnetID, err)
	}

	return nil
}

func (id *EKSInfraDeleter) Delete(infra *EKSInfra) error {
	for _, subnetID := range infra.SubnetIDs {
		if err := id.DeleteSubnet(subnetID); err != nil {
			return fmt.Errorf("error deleting eks infra: %w", err)
		}
	}

	if err := id.DeleteVpc(infra.VpcID); err != nil {
		return fmt.Errorf("error deleting eks infra: %w", err)
	}

	return nil
}

func NewEKSInfraCreator(workDir, region string) *EKSInfraCreator {
	return &EKSInfraCreator{
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: workDir,
			},
		),
		region: region,
	}
}

func (ic *EKSInfraCreator) CreateVpc() (string, error) {
	vpcID, err := ic.awsRunner.Ec2(
		false,
		"create-vpc",
		"--cidr-block",
		"10.0.0.0/16",
		"--query",
		"Vpc.{VpcId:VpcId}",
		"--region",
		ic.region,
	)
	if err != nil {
		return "", fmt.Errorf("error creating vpc: %w", err)
	}

	return vpcID, nil
}

func (ic *EKSInfraCreator) CreateSubnet(vpcID, cidrBlock, availabilityZone string) (string, error) {
	subnetID, err := ic.awsRunner.Ec2(
		false,
		"create-subnet",
		"--vpc-id",
		vpcID,
		"--cidr-block",
		cidrBlock,
		"--availability-zone",
		availabilityZone,
		"--query",
		"Subnet.{SubnetId:SubnetId}",
		"--region",
		ic.region,
	)
	if err != nil {
		return "", fmt.Errorf("error creating subnet: %w", err)
	}

	return subnetID, nil
}

func (ic *EKSInfraCreator) Create() (*EKSInfra, error) {
	infra := &EKSInfra{}

	subnetCidrs := []string{
		"10.0.182.0/24",
		"10.0.172.0/24",
		"10.0.162.0/24",
	}

	vpcID, err := ic.CreateVpc()
	if err != nil {
		return nil, fmt.Errorf("error creating eks infra: %w", err)
	}

	for i, cidr := range subnetCidrs {
		subnetID, err := ic.CreateSubnet(vpcID, cidr, fmt.Sprintf("%s%c", ic.region, 'a'+i))
		if err != nil {
			return nil, fmt.Errorf("error creating eks infra: %w", err)
		}

		infra.SubnetIDs = append(infra.SubnetIDs, subnetID)
	}

	infra.VpcID = vpcID

	return infra, nil
}
