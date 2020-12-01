package configuration

import (
	"reflect"
	"testing"

	bootstrapcfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	clustercfg "github.com/sighupio/furyctl/internal/cluster/configuration"
)

var sampleAWSSimpleConfig Configuration
var sampleDummyConfig Configuration

func init() {
	sampleAWSSimpleConfig.APIVersion = "v0.1.0"
	sampleAWSSimpleConfig.Kind = "Cluster"
	sampleAWSSimpleConfig.Metadata = Metadata{
		Name: "my-cluster",
	}
	sampleAWSSimpleConfig.Provisioner = "aws-simple"
	sampleAWSSimpleConfig.Spec = clustercfg.AWSSimple{
		Provisioner:        "aws-simple",
		Region:             "eu-central-1",
		Version:            "1.18.8",
		PublicSubnetID:     "subnet-2e2fda52",
		PrivateSubnetID:    "subnet-8308f0cf",
		TrustedCIDRs:       []string{"0.0.0.0/0"},
		MasterInstanceType: "m5.large",
		WorkerInstanceType: "m5.large",
		WorkerCount:        1,
		PodNetworkCIDR:     "172.16.0.0/16",
	}

	sampleDummyConfig.APIVersion = "v0.1.0"
	sampleDummyConfig.Kind = "Bootstrap"
	sampleDummyConfig.Metadata = Metadata{
		Name: "my-dummy",
	}
	sampleDummyConfig.Provisioner = "dummy"
	sampleDummyConfig.Spec = bootstrapcfg.Dummy{
		RSABits:     4096,
		Provisioner: "dummy",
	}
}

func TestParseClusterConfigurationFile(t *testing.T) {
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
			name: "AWS Simple",
			args: args{
				path: "assets/sample-config.yml",
			},
			want:    &sampleAWSSimpleConfig,
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
			name: "Invalid kind",
			args: args{
				path: "assets/invalid-kind.yml",
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "Dummy bootstrap",
			args: args{
				path: "assets/dummy-config.yml",
			},
			want:    &sampleDummyConfig,
			wantErr: false,
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
