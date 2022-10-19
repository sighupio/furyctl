// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
)

const ValidLength = 2

func GetSchemaPath(basePath string, conf config.Furyctl) (string, error) {
	avp := strings.Split(conf.APIVersion, "/")

	if len(avp) < ValidLength {
		return "", fmt.Errorf("invalid apiVersion: %s", conf.APIVersion)
	}

	ns := strings.Replace(avp[0], ".sighup.io", "", 1)
	ver := avp[1]

	if conf.Kind == "" {
		return "", fmt.Errorf("kind is empty")
	}

	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind), ns, ver)

	return filepath.Join(basePath, "schemas", filename), nil
}

func GetDefaultsPath(basePath string) string {
	return filepath.Join(basePath, "furyctl-defaults.yaml")
}
