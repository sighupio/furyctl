// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
)

const ValidLength = 2

var (
	errKindIsEmpty       = errors.New("kind is empty")
	errInvalidAPIVersion = errors.New("invalid apiVersion")
)

func GetSchemaPath(basePath string, conf config.Furyctl) (string, error) {
	avp := strings.Split(conf.APIVersion, "/")

	if len(avp) < ValidLength {
		return "", fmt.Errorf("%w: %s", errInvalidAPIVersion, conf.APIVersion)
	}

	ns := strings.Replace(avp[0], ".sighup.io", "", 1)
	ver := avp[1]

	if conf.Kind == "" {
		return "", errKindIsEmpty
	}

	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind), ns, ver)

	return filepath.Join(basePath, "schemas", filename), nil
}

func GetDefaultsPath(basePath string) string {
	return filepath.Join(basePath, "furyctl-defaults.yaml")
}
