// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubex

import (
	"errors"
	"fmt"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var ErrCannotCreateSecret = errors.New("cannot create secret")

func CreateSecret(name, namespace string, data map[string]string) ([]byte, error) {
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
		Data: data,
	}

	secretYaml, err := yamlx.MarshalV3(secret)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrCannotCreateSecret, err)
	}

	return secretYaml, nil
}
