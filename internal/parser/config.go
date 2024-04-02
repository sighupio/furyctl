// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	httpx "github.com/sighupio/furyctl/internal/x/http"
)

const (
	Path  = "path"
	Env   = "env"
	File  = "file"
	HTTP  = "http"
	HTTPS = "https"
)

var (
	ErrCannotParseDynamicValue = fmt.Errorf("cannot parse dynamic value")
	RelativePathRegexp         = regexp.MustCompile(`^\.{1,}\/`)
)

type ConfigParser struct {
	baseDir string
}

func NewConfigParser(baseDir string) *ConfigParser {
	return &ConfigParser{
		baseDir: baseDir,
	}
}

func (p *ConfigParser) ParseDynamicValue(val any) (string, error) {
	strVal := fmt.Sprintf("%v", val)

	spl := strings.Split(strVal, "://")

	if len(spl) > 1 {
		source := strings.TrimPrefix(spl[0], "{")
		sourceValue := strings.TrimSuffix(spl[1], "}")

		switch source {
		case Path:
			return filepath.Join(p.baseDir, filepath.Clean(sourceValue)), nil

		case Env:
			envVar := os.Getenv(sourceValue)

			envVar = strings.TrimRight(envVar, "\n")

			return envVar, nil

		case File:
			// If the value is a relative path, we need to convert it to an absolute path.
			isRelativePath := RelativePathRegexp.MatchString(sourceValue)
			if isRelativePath {
				sourceValue = filepath.Clean(sourceValue)
				sourceValue = filepath.Join(p.baseDir, sourceValue)
			}

			val, err := os.ReadFile(sourceValue)
			if err != nil {
				return "", fmt.Errorf("%w: %w", ErrCannotParseDynamicValue, err)
			}

			return strings.TrimRight(string(val), "\n"), nil

		case HTTP, HTTPS:
			f, err := httpx.DownloadFile(strings.Trim(strVal, "{}"))
			if err != nil {
				return "", fmt.Errorf("%w: %w", ErrCannotParseDynamicValue, err)
			}

			val, err := os.ReadFile(f)
			if err != nil {
				return "", fmt.Errorf("%w: %w", ErrCannotParseDynamicValue, err)
			}

			return strings.TrimRight(string(val), "\n"), nil

		default:
			return strVal, nil
		}
	}

	return strVal, nil
}
