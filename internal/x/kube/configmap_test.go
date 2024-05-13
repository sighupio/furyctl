// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubex_test

import (
	"testing"

	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

func TestCreateConfigMap(t *testing.T) {
	t.Parallel()

	name := "test"
	namespace := "test"
	key := "config"
	config := "test"

	secret := struct {
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
			"config": config,
		},
	}

	want, err := yamlx.MarshalV3(secret)
	if err != nil {
		t.Fatal(err)
	}

	got, err := kubex.CreateConfigMap([]byte("test"), name, key, namespace)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(want) {
		t.Fatalf("got %s, want %s", string(got), string(want))
	}
}
