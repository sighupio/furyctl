// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package furyagent

import (
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Storage struct {
	BucketName string `yaml:"bucketName"`
}

type AgentConfig struct {
	Storage          Storage        `yaml:"storage"`
	ClusterComponent map[string]any `yaml:"clusterComponent"`
}

func ParseConfig(cfgPath string) (*AgentConfig, error) {
	cfg, err := yamlx.FromFileV3[*AgentConfig](cfgPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
