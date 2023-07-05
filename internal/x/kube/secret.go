// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubex

import (
	"encoding/base64"
	"fmt"

	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

func CreateSecret(data []byte, name, key, namespace string) ([]byte, error) {
	secret := struct {
		APIVersion string            `yaml:"apiVersion"`
		Kind       string            `yaml:"kind"`
		Metadata   map[string]any    `yaml:"metadata"`
		Type       string            `yaml:"type"`
		Data       map[string]string `yaml:"data"`
	}{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata: map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		Type: "Opaque",
		Data: map[string]string{
			key: base64.StdEncoding.EncodeToString(data),
		},
	}

	data, err := yamlx.MarshalV3(secret)
	if err != nil {
		return []byte{}, fmt.Errorf("%w", err)
	}

	return data, nil
}
