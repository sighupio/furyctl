// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package santhosh

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

var filepathRefRegexp = regexp.MustCompile(`\"\$ref":\ *"(\.{1,2}\/.+)"`)

var ErrCannotLoadSchema = errors.New("failed to load schema file")

func LoadSchema(schemaPath string) (*jsonschema.Schema, error) {
	berr := fmt.Errorf("%w '%s'", ErrCannotLoadSchema, schemaPath)

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", berr, err)
	}

	data = filepathRefRegexp.ReplaceAll(data, []byte(fmt.Sprintf(`"$$ref": "file://%s/$1"`, filepath.Dir(schemaPath))))

	compiler := jsonschema.NewCompiler()

	if err = compiler.AddResource(schemaPath, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("%w: %v", berr, err)
	}

	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", berr, err)
	}

	return schema, nil
}
