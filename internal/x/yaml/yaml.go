// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package yamlx

import (
	"fmt"
	"os"

	v2 "gopkg.in/yaml.v2"
	v3 "gopkg.in/yaml.v3"
)

func FromFileV2[T any](path string) (T, error) {
	var data T

	res, err := os.ReadFile(path)
	if err != nil {
		return data, fmt.Errorf("error while reading file from %s :%w", path, err)
	}

	if err := v2.Unmarshal(res, &data); err != nil {
		return data, fmt.Errorf("error while unmarshalling file from %s :%w", path, err)
	}

	return data, nil
}

func MarshalV2(in any) ([]byte, error) {
	out, err := v2.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling yaml: %w", err)
	}

	return out, nil
}

func FromFileV3[T any](file string) (T, error) {
	var data T

	res, err := os.ReadFile(file)
	if err != nil {
		return data, fmt.Errorf("error while reading file from %s :%w", file, err)
	}

	if err := v3.Unmarshal(res, &data); err != nil {
		return data, fmt.Errorf("error while unmarshalling file from %s :%w", file, err)
	}

	return data, nil
}

func MarshalV3(in any) ([]byte, error) {
	out, err := v3.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling yaml: %w", err)
	}

	return out, nil
}
