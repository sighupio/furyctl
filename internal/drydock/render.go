// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package drydock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"

	parserx "github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

var ErrTemplate = errors.New("wizard template error")

// Render executes a wizard template with the answers. A failure here is a bug in the wizard
// files, never a user mistake, so it is a distinct error the UI shows as such.
func Render(tpl string, answers map[string]any) (string, error) {
	funcs := templatex.NewFuncMap()
	funcs.Add("toYaml", templatex.ToYAML)

	t, err := template.New("furyctl.yaml").Funcs(funcs.FuncMap).Option("missingkey=default").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTemplate, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, answers); err != nil {
		return "", fmt.Errorf("%w: %w", ErrTemplate, err)
	}

	return buf.String(), nil
}

// FieldError is one schema violation, located by a JSON pointer into the document.
type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Validate checks a rendered document against a distribution schema. Values that furyctl
// expands at apply time (such as {env://NAME} and {file://PATH}) cannot satisfy patterns
// before expansion, so violations on those values are dropped: the user has not exported them yet.
func Validate(schemaPath, doc string) ([]FieldError, error) {
	var parsed any
	if err := yaml.Unmarshal([]byte(doc), &parsed); err != nil {
		return nil, fmt.Errorf("parsing rendered yaml: %w", err)
	}

	// Round-trip through JSON so numbers and maps have the types the validator expects.
	jb, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("encoding rendered yaml: %w", err)
	}

	var instance any
	if err := json.Unmarshal(jb, &instance); err != nil {
		return nil, fmt.Errorf("decoding rendered yaml: %w", err)
	}

	schema, err := santhosh.LoadSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("loading schema: %w", err)
	}

	err = schema.Validate(instance)
	if err == nil {
		return nil, nil
	}

	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return nil, fmt.Errorf("validating rendered yaml: %w", err)
	}

	var out []FieldError

	for _, leaf := range leaves(verr) {
		if isDynamic(instance, leaf.InstanceLocation) {
			continue
		}

		out = append(out, FieldError{Path: leaf.InstanceLocation, Message: leaf.Message})
	}

	return out, nil
}

func leaves(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}

	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, leaves(c)...)
	}

	return out
}

// isDynamic reports whether the value at a JSON pointer is a string furyctl will expand later.
func isDynamic(instance any, pointer string) bool {
	cur := instance

	for seg := range strings.SplitSeq(strings.TrimPrefix(pointer, "/"), "/") {
		if seg == "" {
			continue
		}

		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")

		switch node := cur.(type) {
		case map[string]any:
			cur = node[seg]

		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(node) {
				return false
			}

			cur = node[i]

		default:
			return false
		}
	}

	s, ok := cur.(string)

	return ok && parserx.DynamicRegexp.MatchString(s)
}
