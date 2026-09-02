// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package drydock

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidWhen = errors.New("invalid when expression")

// Cond is a parsed `when` expression: an OR of ANDs of terms.
//
// Grammar: expr := and ("||" and)* ; and := term ("&&" term)* ;
// term := ident | "!" ident | ident "==" literal | ident "!=" literal.
//
// Identifiers are dotted paths; a leading segment that names a step in root is
// resolved from root, everything else from the current step's scope.
// Literals are bare words or numbers, compared as strings. That is all.
type Cond struct {
	ors [][]term
}

type term struct {
	path  string
	op    string // One of "truthy", "falsy", "eq", "ne".
	value string
}

func ParseWhen(s string) (Cond, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Cond{}, nil
	}

	var cond Cond

	for orPart := range strings.SplitSeq(s, "||") {
		var ands []term

		for andPart := range strings.SplitSeq(orPart, "&&") {
			t, err := parseTerm(strings.TrimSpace(andPart))
			if err != nil {
				return Cond{}, err
			}

			ands = append(ands, t)
		}

		cond.ors = append(cond.ors, ands)
	}

	return cond, nil
}

func parseTerm(s string) (term, error) {
	if s == "" {
		return term{}, fmt.Errorf("%w: empty term", ErrInvalidWhen)
	}

	for _, op := range []string{"==", "!="} {
		left, right, ok := strings.Cut(s, op)
		if !ok {
			continue
		}

		left, right = strings.TrimSpace(left), strings.TrimSpace(right)
		if !isIdent(left) || right == "" || strings.ContainsAny(right, " \t") {
			return term{}, fmt.Errorf("%w: %q", ErrInvalidWhen, s)
		}

		name := "eq"
		if op == "!=" {
			name = "ne"
		}

		return term{path: left, op: name, value: right}, nil
	}

	if neg, ok := strings.CutPrefix(s, "!"); ok {
		neg = strings.TrimSpace(neg)
		if !isIdent(neg) {
			return term{}, fmt.Errorf("%w: %q", ErrInvalidWhen, s)
		}

		return term{path: neg, op: "falsy"}, nil
	}

	if !isIdent(s) {
		return term{}, fmt.Errorf("%w: %q", ErrInvalidWhen, s)
	}

	return term{path: s, op: "truthy"}, nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		ok := r == '.' || r == '_' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}

	return true
}

// Eval returns true when any AND-group holds. An empty Cond is always true.
func (c Cond) Eval(scope, root map[string]any) bool {
	if len(c.ors) == 0 {
		return true
	}

	for _, ands := range c.ors {
		all := true

		for _, t := range ands {
			if !t.holds(scope, root) {
				all = false

				break
			}
		}

		if all {
			return true
		}
	}

	return false
}

func (t term) holds(scope, root map[string]any) bool {
	v, found := lookup(t.path, scope, root)

	switch t.op {
	case "truthy":
		return found && truthy(v)

	case "falsy":
		return !found || !truthy(v)

	case "eq":
		return found && fmt.Sprint(v) == t.value

	case "ne":
		return !found || fmt.Sprint(v) != t.value

	default:
		return false
	}
}

func lookup(path string, scope, root map[string]any) (any, bool) {
	parts := strings.Split(path, ".")

	var cur any = scope
	if _, isStep := root[parts[0]]; isStep {
		cur = root
	}

	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}

		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}

	return cur, true
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false

	case bool:
		return x

	case string:
		return x != ""

	case float64:
		return x != 0

	case int:
		return x != 0

	case []any:
		return len(x) > 0

	default:
		return true
	}
}
