// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configuration

import (
	"reflect"
	"testing"

	bootstrapcfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	clustercfg "github.com/sighupio/furyctl/internal/cluster/configuration"
)

var sampleEKSConfig Configuration
var sampleAWSBootstrap Configuration

func init() {

	sampleAWSBootstrap.Kind = "Bootstrap"
	sampleAWSBootstrap.Metadata = Metadata{
		Name: "my-aws-poc",
	}
	sampleAWSBootstrap.Provisioner = "aws"
	sampleAWSBootstrap.Spec = bootstrapcfg.AWS{
		NetworkCIDR:         "10.0.0.0/16",
		PublicSubnetsCIDRs:  []string{"10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"},
		PrivateSubnetsCIDRs: []string{"10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"},
		VPN: bootstrapcfg.AWSVPN{
			Instances:     1,
			InstanceType:  "t3.large",
			Port:          1194,
			DiskSize:      50,
			DHParamsBits:  2048,
			SubnetCIDR:    "192.168.100.0/24",
			SSHUsers:      []string{"angelbarrera92"},
			OperatorName:  "sighup",
			OperatorCIDRs: []string{"1.2.3.4/32"},
		},
	}

	sampleEKSConfig.Kind = "Cluster"
	sampleEKSConfig.Metadata = Metadata{
		Name: "demo",
	}
	sampleEKSConfig.Executor.Version = "0.12.29"
	sampleEKSConfig.Executor.StateConfiguration = StateConfiguration{
		Backend: "s3",
		Config: map[string]string{
			"bucket": "terraform-e2e-fury-testing-angel",
			"key":    "cli/demo/cluster",
			"region": "eu-central-1",
		},
	}
	sampleEKSConfig.Provisioner = "eks"
	sampleEKSConfig.Spec = clustercfg.EKS{
		Version:      "1.18",
		Network:      "vpc-1",
		SubNetworks:  []string{"subnet-1", "subnet-2", "subnet-3"},
		DMZCIDRRange: clustercfg.DMZCIDRRange{Values: []string{"0.0.0.0/0"}},
		SSHPublicKey: "123",
		NodePools: []clustercfg.EKSNodePool{
			{
				Name:         "one",
				Version:      "1.18",
				MinSize:      0,
				MaxSize:      10,
				InstanceType: "m",
				MaxPods:      100,
				VolumeSize:   50,
				Labels: map[string]string{
					"hello": "World",
				},
				Taints: []string{"hello"},
				Tags: map[string]string{
					"hello": "World",
				},
			},
		},
	}
}

func TestParseClusterConfigurationFile(t *testing.T) {

	sampleAWSBootstrapLocalState := sampleAWSBootstrap
	sampleAWSBootstrapLocalState.Executor.StateConfiguration.Backend = "local"
	sampleAWSBootstrapLocalState.Executor.StateConfiguration.Config = map[string]string{
		"path": "mystate.tfstate",
	}

	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    *Configuration
		wantErr bool
	}{
		{
			name: "AWS config",
			args: args{
				path: "assets/aws-bootstrap.yml",
			},
			want:    &sampleAWSBootstrap,
			wantErr: false,
		},
		{
			name: "AWS config - File State",
			args: args{
				path: "assets/aws-bootstrap-file-state.yml",
			},
			want:    &sampleAWSBootstrapLocalState,
			wantErr: false,
		},
		{
			name: "EKS config",
			args: args{
				path: "assets/eks-cluster.yml",
			},
			want:    &sampleEKSConfig,
			wantErr: false,
		},
		{
			name: "Undefined provisioner",
			args: args{
				path: "assets/invalid-config.yml",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Undefined provisioner for bootstrap",
			args: args{
				path: "assets/invalid-provisioner-bootstrap.yml",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid kind",
			args: args{
				path: "assets/invalid-kind.yml",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseClusterConfigurationFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseClusterConfigurationFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
