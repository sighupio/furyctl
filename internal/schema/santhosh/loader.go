package santhosh

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
)

var (
	ErrCannotLoadSchema = errors.New("failed to read schema file")
)

func LoadSchema(schemaPath string) (schema *jsonschema.Schema, errSchema error) {
	berr := fmt.Errorf("%w: '%s'", ErrCannotLoadSchema, schemaPath)

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", berr, err)
	}

	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource(schemaPath, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("%w: %v", berr, err)
	}

	schema, errSchema = compiler.Compile(schemaPath)
	if errSchema != nil {
		return nil, fmt.Errorf("%w: %v", berr, err)
	}

	return schema, nil
}
