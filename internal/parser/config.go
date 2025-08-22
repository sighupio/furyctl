// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"errors"
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
	ErrCannotParseDynamicValue = errors.New("cannot parse dynamic value")
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

func (p *ConfigParser) ParseDynamicValue(val any) (any, error) {
	// Handle different types appropriately.
	switch v := val.(type) {
	case string:
		// Check if this string contains dynamic value patterns.
		if !strings.Contains(v, "://") {
			// No dynamic pattern, return as-is.
			return v, nil
		}

		return p.parseDynamicString(v)

	case []any:
		// Process each element in the array.
		result := make([]any, len(v))

		for i, item := range v {
			processedItem, err := p.ParseDynamicValue(item)
			if err != nil {
				return nil, fmt.Errorf("error processing array element %d: %w", i, err)
			}

			result[i] = processedItem
		}

		return result, nil

	case []string:
		// Process each string in the array.
		result := make([]any, len(v))

		for i, item := range v {
			processedItem, err := p.ParseDynamicValue(item)
			if err != nil {
				return nil, fmt.Errorf("error processing array element %d: %w", i, err)
			}

			result[i] = processedItem
		}

		return result, nil

	default:
		// For other types (bool, int, float, etc.), return as-is.
		return val, nil
	}
}

// parseDynamicString processes a string that may contain dynamic value patterns.
func (p *ConfigParser) parseDynamicString(strVal string) (string, error) {
	spl := strings.Split(strVal, "://")

	if len(spl) > 1 {
		source := strings.TrimPrefix(spl[0], "{")
		sourceValue := strings.TrimSuffix(spl[1], "}")

		switch source {
		case Path:
			return filepath.Join(p.baseDir, filepath.Clean(sourceValue)), nil

		case Env:
			envVar, exists := os.LookupEnv(sourceValue)
			if !exists {
				return "", fmt.Errorf("%w: \"%s\" is empty", ErrCannotParseDynamicValue, sourceValue)
			}

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
