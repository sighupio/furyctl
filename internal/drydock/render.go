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

// CheckYAML parses what the user typed into every `yaml` field of the wizard, so a mistake is
// reported against the field that holds it instead of breaking the whole rendered document with
// a message about a line number nobody can place.
func CheckYAML(w *Wizard, answers map[string]any) []FieldError {
	out := make([]FieldError, 0, len(w.Steps))

	for _, s := range w.Steps {
		scope, ok := answers[s.ID].(map[string]any)
		if !ok {
			continue
		}

		out = append(out, checkYAMLFields(s.ID, s.Fields, scope)...)
	}

	return out
}

func checkYAMLFields(path string, fields []Field, scope map[string]any) []FieldError {
	var out []FieldError

	for i := range fields {
		f := &fields[i]
		where := path + "." + f.ID

		switch f.Type {
		case "yaml":
			text, ok := scope[f.ID].(string)
			if !ok || strings.TrimSpace(text) == "" {
				continue
			}

			var parsed any
			if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
				out = append(out, FieldError{Path: where, Message: "is not valid YAML: " + err.Error()})
			}

		case "group":
			if nested, ok := scope[f.ID].(map[string]any); ok {
				out = append(out, checkYAMLFields(where, f.Fields, nested)...)
			}

		case "list":
			items, ok := scope[f.ID].([]any)
			if !ok {
				continue
			}

			for j, item := range items {
				nested, ok := item.(map[string]any)
				if !ok {
					continue
				}

				out = append(out, checkYAMLFields(fmt.Sprintf("%s.%d", where, j), f.Fields, nested)...)
			}

		default:
		}
	}

	return out
}

// FieldError is one schema violation, located by a JSON pointer into the document.
// Missing marks violations that only mean "not filled in yet": a required key that is
// absent, or an empty string. The UI counts those calmly; the rest are real mistakes.
type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Missing bool   `json:"missing"`
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

		out = append(out, FieldError{
			Path:    leaf.InstanceLocation,
			Message: leaf.Message,
			Missing: strings.HasSuffix(leaf.KeywordLocation, "/required") || isEmpty(instance, leaf.InstanceLocation),
		})
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
	s, ok := valueAt(instance, pointer).(string)

	return ok && parserx.DynamicRegexp.MatchString(s)
}

// isEmpty reports whether the value at a JSON pointer is absent, nil or an empty string.
func isEmpty(instance any, pointer string) bool {
	switch v := valueAt(instance, pointer).(type) {
	case nil:
		return true

	case string:
		return v == ""

	default:
		return false
	}
}

// valueAt resolves a JSON pointer against a decoded document; nil when the path does not exist.
func valueAt(instance any, pointer string) any {
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
				return nil
			}

			cur = node[i]

		default:
			return nil
		}
	}

	return cur
}
