// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package secrets

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/sirupsen/logrus"

	distroconf "github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

var (
	// ErrNoField reports a component that the cluster has no field to hold the secrets of. Both the
	// kind of the cluster and the version of the distribution decide which fields a cluster has.
	ErrNoField = errors.New("the cluster has no field for the component")

	// ErrUnreadableSchema reports a schema file that furyctl can read as JSON, but cannot walk. Such
	// a schema answers no to every field, thus it would drop the secrets of each component.
	ErrUnreadableSchema = errors.New("cannot read the fields of the schema")
)

// The field that tells whether a schema answers about a cluster at all: see LoadSchema. Each kind
// holds it, and it is one field deep on purpose. A stub that holds `spec` and takes no field in it
// answers no to every secret, and a stub that takes no field at all answers no to this one too.
const anchorField = "spec.distribution"

// Schema is the public schema of one kind of cluster, at one version of the distribution. It tells
// which fields that cluster has, thus which secrets furyctl can write a reference to.
//
// A field arrives in one version of the distribution, and it can go away in another one. The kinds
// do not hold the same fields either. The schema of a version answers for that version alone, thus
// furyctl keeps no table of versions and no list of kinds.
type Schema struct {
	root *jsonschema.Schema
}

// LoadSchema reads the public schema of the kind of a configuration file from a distribution.
func LoadSchema(repoPath string, conf distroconf.Furyctl) (*Schema, error) {
	path, err := distribution.GetPublicSchemaPath(repoPath, conf)
	if err != nil {
		return nil, fmt.Errorf("error while building the path of the schema: %w", err)
	}

	root, err := santhosh.LoadSchema(path)
	if err != nil {
		return nil, fmt.Errorf("error while loading the schema: %w", err)
	}

	schema := &Schema{root: root}

	// A schema that turns each field away is valid JSON, and it answers no to every field. Such an
	// answer would drop the secrets of each component, and the command would report a cluster that
	// needs no secret. An answer of no about the field that each kind holds thus says nothing about
	// the cluster: it says that furyctl cannot read this schema.
	if !schema.Has(anchorField) {
		return nil, fmt.Errorf("%w: %s turns the %s field away", ErrUnreadableSchema, path, anchorField)
	}

	return schema, nil
}

// Has reports whether the cluster can hold the field at the given path. A nil Schema answers yes to
// every path: furyctl cannot read the schema of the cluster, thus it writes the secret anyway. An
// extra secret is a file that nothing reads. A missing secret stops the cluster.
//
// The answer comes from the schema itself: furyctl builds a document that holds one value at the
// path, and asks the schema to validate it. The answer is thus the answer that `furyctl validate
// config` gives, and the two commands cannot disagree.
func (s *Schema) Has(path string) bool {
	// A path with no field in it is the answer of a caller that asks about nothing. An answer of no
	// would drop a secret over a fault of furyctl, thus the answer stays on the safe side.
	if s == nil || s.root == nil || path == "" {
		return true
	}

	err := s.root.Validate(plant(path))
	if err == nil {
		return true
	}

	var invalid *jsonschema.ValidationError
	if !errors.As(err, &invalid) {
		return true
	}

	return !turnsAway(invalid, pointer(path))
}

// plant builds a document that holds one value at the path, and nothing else. The value is a string
// because no field of a secret holds anything else, and because the answer comes from the name of
// the field and not from its type: see turnsAway.
func plant(path string) any {
	var document any = "furyctl"

	for _, field := range slices.Backward(strings.Split(path, ".")) {
		document = map[string]any{field: document}
	}

	return document
}

// pointer returns the path as the JSON pointer that a schema reports. The schema escapes each part
// of a pointer with `~0`, `~1` and the escapes of a URL path, thus a field whose name holds a `/`,
// a `~`, a `%` or a space gets no answer from here. A `.` in a name gets none either, because it
// separates the fields of a path. The fields in All hold none of them.
func pointer(path string) string {
	return "/" + strings.ReplaceAll(path, ".", "/")
}

// turnsAway reports whether the schema turned the field away by its name, and not some other part
// of the document. An `additionalProperties` of false is the keyword that turns a name away, and
// each public schema of the distribution holds it more than a hundred times. Every other fault, for
// example the type of the value or a field that the schema asks for, says that the name is one that
// the cluster can hold.
func turnsAway(err *jsonschema.ValidationError, location string) bool {
	if fromABranch(err.KeywordLocation) {
		return false
	}

	if strings.HasSuffix(err.KeywordLocation, "/additionalProperties") && covers(err.InstanceLocation, location) {
		return true
	}

	return lo.SomeBy(err.Causes, func(cause *jsonschema.ValidationError) bool {
		return turnsAway(cause, location)
	})
}

// fromABranch reports whether the fault comes from a schema that some clusters match and others do
// not. The document that plant builds holds one field at each step and no more, thus it matches no
// `required` and each branch that a `required` guards takes the way of the clusters that furyctl is
// not asking about. A fault from such a branch says nothing about the field.
//
// The `allOf` keyword is not here: each schema of an `allOf` holds for every cluster.
func fromABranch(keyword string) bool {
	return lo.SomeBy([]string{"/anyOf/", "/oneOf/", "/if/", "/then/", "/else/", "/not/"},
		func(branch string) bool { return strings.Contains(keyword, branch) })
}

// covers reports whether the object at the first pointer holds the field at the second one. The
// comparison follows the parts of a pointer: the object at `/spec` holds `/spec/kubernetes`, and the
// object at `/spe` holds nothing of it.
func covers(object, field string) bool {
	return object == "" || field == object || strings.HasPrefix(field, object+"/")
}

// Keep returns the components with only the secrets that the cluster has a field for, and drops
// each component that keeps no secret. It reports the name of every component that it dropped: a
// component that the user named is an error, while one that furyctl chose is only a component that
// this cluster cannot use.
func Keep(components []Component, schema *Schema) ([]Component, []string) {
	kept := make([]Component, 0, len(components))
	dropped := make([]string, 0, len(components))

	for _, component := range components {
		component.Secrets = lo.Filter(component.Secrets, func(s Secret, _ int) bool {
			if schema.Has(s.Path) {
				return true
			}

			// The user must read this. A secret that furyctl leaves out is a secret that the
			// cluster does not get, and only this line tells the difference between a field that
			// the version of the distribution does not hold and a fault of furyctl.
			logrus.Infof(
				"This cluster has no %s field, thus furyctl creates no %s secret for %s.",
				s.Path, s.Name, component.Name,
			)

			return false
		})

		if len(component.Secrets) == 0 {
			dropped = append(dropped, component.Name)

			continue
		}

		kept = append(kept, component)
	}

	return kept, dropped
}
