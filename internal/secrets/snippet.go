// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package secrets

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// snippetIndent is the number of spaces of each level of the YAML that furyctl writes.
const snippetIndent = 2

// Reference returns the value that a field of the configuration file needs to read the file of a
// secret. The `{file://}` notation reads a relative path from the folder of the configuration file,
// thus the path must be relative to that folder.
func Reference(file, configDir string) string {
	return "{file://" + filepath.ToSlash(relative(file, configDir)) + "}"
}

// relative returns the path of the file from the folder, with the "./" prefix that furyctl needs to
// resolve it. It makes both paths absolute first, thus one of the two can be relative to the
// current folder and the other one not. When the two paths have no folder in common, it returns the
// path of the file as it is. An absolute path needs no folder to resolve it.
func relative(file, folder string) string {
	absFile, fileErr := filepath.Abs(file)
	absFolder, folderErr := filepath.Abs(folder)

	if fileErr != nil || folderErr != nil {
		return file
	}

	path, err := filepath.Rel(absFolder, absFile)
	if err != nil {
		return file
	}

	// A path that starts with ".." needs no prefix: furyctl resolves it already.
	if !strings.HasPrefix(path, ".") {
		return "./" + path
	}

	return path
}

// Snippet returns the fields to add to the configuration file, in YAML, with a reference to the
// file of each secret.
func Snippet(results []Result, configDir string) (string, error) {
	root := map[string]any{}

	for _, result := range results {
		fields := strings.Split(result.Path, ".")
		parent := root

		for _, field := range fields[:len(fields)-1] {
			child, ok := parent[field].(map[string]any)
			if !ok {
				child = map[string]any{}
				parent[field] = child
			}

			parent = child
		}

		parent[fields[len(fields)-1]] = Reference(result.File, configDir)
	}

	var out bytes.Buffer

	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(snippetIndent)

	if err := encoder.Encode(root); err != nil {
		return "", fmt.Errorf("error while writing the YAML of the secrets: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("error while writing the YAML of the secrets: %w", err)
	}

	return out.String(), nil
}
