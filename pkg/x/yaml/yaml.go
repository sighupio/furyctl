// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package yamlx

import (
	"fmt"
	"os"

	yamlv2 "go.yaml.in/yaml/v2"
	yamlv3 "go.yaml.in/yaml/v3"
)

func FromFileV2[T any](path string) (T, error) {
	var data T

	res, err := os.ReadFile(path)
	if err != nil {
		return data, fmt.Errorf("error while reading file from %s :%w", path, err)
	}

	if err := yamlv2.Unmarshal(res, &data); err != nil {
		return data, fmt.Errorf("error while unmarshalling file from %s :%w", path, err)
	}

	return data, nil
}

func MarshalV2(in any) ([]byte, error) {
	out, err := yamlv2.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling yaml: %w", err)
	}

	return out, nil
}

func UnmarshalV2(in []byte, out any) error {
	if err := yamlv2.Unmarshal(in, out); err != nil {
		return fmt.Errorf("error while unmarshalling yaml: %w", err)
	}

	return nil
}

func FromFileV3[T any](file string) (T, error) {
	var data T

	res, err := os.ReadFile(file)
	if err != nil {
		return data, fmt.Errorf("error while reading file from %s :%w", file, err)
	}

	if err := yamlv3.Unmarshal(res, &data); err != nil {
		return data, fmt.Errorf("error while unmarshalling file from %s :%w", file, err)
	}

	return data, nil
}

func MarshalV3(in any) ([]byte, error) {
	out, err := yamlv3.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling yaml: %w", err)
	}

	return out, nil
}

func UnmarshalV3(in []byte, out any) error {
	if err := yamlv3.Unmarshal(in, out); err != nil {
		return fmt.Errorf("error while unmarshalling yaml: %w", err)
	}

	return nil
}
