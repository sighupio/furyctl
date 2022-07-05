// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package yaml

import (
	"gopkg.in/yaml.v3"
	"os"
)

func FromFile[T any](path string) (T, error) {
	var yamlRes T

	res, err := os.ReadFile(path)
	if err != nil {
		return yamlRes, err
	}

	err = yaml.Unmarshal(res, &yamlRes)

	return yamlRes, err

}
