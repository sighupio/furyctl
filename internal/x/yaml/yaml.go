// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package yamlx

import (
	"os"

	v2 "gopkg.in/yaml.v2"
	v3 "gopkg.in/yaml.v3"
)

func FromFileV2[T any](path string) (T, error) {
	var data T

	res, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}

	if err := v2.Unmarshal(res, &data); err != nil {
		return data, err
	}

	return data, nil
}

func MarshalV2(in any) ([]byte, error) {
	return v2.Marshal(in)
}

func FromFileV3[T any](file string) (T, error) {
	var data T

	res, err := os.ReadFile(file)
	if err != nil {
		return data, err
	}

	if err := v3.Unmarshal(res, &data); err != nil {
		return data, err
	}

	return data, nil
}

func MarshalV3(in any) ([]byte, error) {
	return v3.Marshal(in)
}
