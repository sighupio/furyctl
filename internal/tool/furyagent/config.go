// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package furyagent

import (
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type Storage struct {
	BucketName   string `yaml:"bucketName"`
	Provider     string `yaml:"provider"`
	AwsAccessKey string `yaml:"aws_access_key"` //nolint:tagliatelle // yaml key is aws_access_key
	AwsSecretKey string `yaml:"aws_secret_key"` //nolint:tagliatelle // yaml key is aws_secret_key
	Region       string `yaml:"region"`
}

type ClusterComponent struct {
	OpenVPN OpenVPN `yaml:"openvpn"`
	SSHKeys SSHKeys `yaml:"sshkeys"`
}

type OpenVPN struct {
	CertDir string   `yaml:"certDir"`
	Servers []string `yaml:"servers"`
}

type SSHKeys struct {
	User            string  `yaml:"user"`
	TempDir         string  `yaml:"tempDir"`
	LocalDirConfigs string  `yaml:"localDirConfigs"`
	Adapter         Adapter `yaml:"adapter"`
}

type Adapter struct {
	Name string `yaml:"name"`
}

type AgentConfig struct {
	Storage          Storage          `yaml:"storage"`
	ClusterComponent ClusterComponent `yaml:"clusterComponent"`
}

func ParseConfig(cfgPath string) (*AgentConfig, error) {
	cfg, err := yamlx.FromFileV3[*AgentConfig](cfgPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
