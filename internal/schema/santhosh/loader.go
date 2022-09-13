package santhosh

import (
	"bytes"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema"
)

func LoadSchema(schemaPath string) (schema *jsonschema.Schema, errSchema error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource(schemaPath, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("failed to add resource to json schema compiler: %w", err)
	}

	schema, errSchema = compiler.Compile(schemaPath)
	if errSchema != nil {
		return nil, fmt.Errorf("failed to compile json schema: %w", err)
	}

	return schema, nil
}
