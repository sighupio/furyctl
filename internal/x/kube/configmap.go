// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubex

import (
	"fmt"

	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

func CreateConfigMap(data []byte, name, key, namespace string) ([]byte, error) {
	configMap := struct {
		APIVersion string            `yaml:"apiVersion"`
		Kind       string            `yaml:"kind"`
		Metadata   map[string]any    `yaml:"metadata"`
		Data       map[string]string `yaml:"data"`
	}{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		Data: map[string]string{
			key: string(data),
		},
	}

	data, err := yamlx.MarshalV3(configMap)
	if err != nil {
		return []byte{}, fmt.Errorf("%w", err)
	}

	return data, nil
}
