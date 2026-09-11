// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"
	"gopkg.in/yaml.v3"

	parserx "github.com/sighupio/furyctl/internal/parser"
)

// The fields that furyctl reads to decide which secrets a cluster needs. Each field that furyctl
// writes is in the row of its secret in All instead, because that row holds the name of the file and
// the generator next to the path. A field that furyctl reads and writes has its name here, thus its
// two uses cannot drift apart. A path that holds the name of a module is built where that name is:
// see moduleType.
const (
	// The field that holds the kind of the cluster. The fields of a kind are not the fields of
	// another one: see resolve.
	kindField = "kind"

	// The field that tells which authentication the infrastructural ingresses use.
	authProviderType = "spec.distribution.modules.auth.provider.type"

	// The field that tells whether the cluster deploys Gangplank.
	oidcKubernetesAuthEnabled = "spec.distribution.modules.auth.oidcKubernetesAuth.enabled"

	// The field that tells whether the cluster deploys HAProxy.
	onPremisesLoadBalancersEnabled = "spec.kubernetes.loadBalancers.enabled"

	// The fields that tell whether the load balancers share a virtual IP. The OnPremises and the
	// Immutable kinds hold that value at fields of their own.
	onPremisesLoadBalancersKeepalivedEnabled = "spec.kubernetes.loadBalancers.keepalived.enabled"
	immutableLoadBalancersKeepalivedEnabled  = "spec.infrastructure.loadBalancers.keepalived.enabled"

	// The field that tells whether the control plane nodes share a virtual IP. Only the Immutable
	// kind has it: the virtual IP is an alternative to the load balancers for the API server.
	immutableControlPlaneKeepalivedEnabled = "spec.kubernetes.controlPlane.keepalived.enabled"

	// The fields that tell where the engine of a module writes its data. MinIO is the default
	// backend, thus a field with no value is a MinIO backend.
	mimirBackend = "spec.distribution.modules.monitoring.mimir.backend"
	tempoBackend = "spec.distribution.modules.tracing.tempo.backend"

	// The field that holds the encryption at rest configuration of etcd. It is the one field that
	// furyctl both writes and reads: an empty field is the answer that the user has not given yet.
	// The OnPremises and the Immutable kinds both have it.
	etcdEncryptionConfiguration = "spec.kubernetes.advanced.encryption.configuration"
)

// Config holds the fields of a configuration file that tell which components a cluster uses.
type Config struct {
	root map[string]any

	// The folder of the configuration file. A `{file://./path}` value resolves from this folder.
	dir string

	// The fields whose dynamic value furyctl cannot read: see Unresolved.
	unresolved []string
}

// LoadConfig reads the configuration file. It returns an error that wraps [os.ErrNotExist] when the
// file is not there: furyctl creates the secrets of every component in that case.
func LoadConfig(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error while reading the configuration file %s: %w", path, err)
	}

	root := map[string]any{}

	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("error while parsing the configuration file %s: %w", path, err)
	}

	return &Config{root: root, dir: filepath.Dir(path)}, nil
}

// String returns the value of a string field, or an empty string when the field is absent.
//
// A field that holds a map, a number or a boolean is not an empty field, thus it goes to Unresolved
// with the fields that furyctl cannot read. An empty answer would make furyctl read such a field as
// a field with no value, and write a second secret over the one that the cluster runs with. The
// schema of a cluster asks for a string in each of these fields, and a user who writes the
// encryption at rest without the `|` of a block scalar writes a map.
func (c *Config) String(path string) string {
	value := c.value(path)
	if value == nil {
		return ""
	}

	text, ok := value.(string)
	if !ok {
		c.unresolved = append(c.unresolved, path)

		return ""
	}

	return c.expand(path, text)
}

// Bool returns the value of a boolean field, or false when the field is absent.
//
// The schema of a cluster asks for a boolean in each of these fields and takes no text: `furyctl
// validate config` reports `enabled: "true"`, `enabled: off` and `enabled: "{env://LB}"` alike,
// because yaml.v3 gives each of them as text. A field that holds anything but a boolean thus holds
// no answer, and Unresolved names it.
func (c *Config) Bool(path string) bool {
	value := c.value(path)
	if value == nil {
		return false
	}

	answer, ok := value.(bool)
	if !ok {
		c.unresolved = append(c.unresolved, path)

		return false
	}

	return answer
}

// Unresolved returns the fields whose dynamic value furyctl cannot read. An environment variable
// with no value and a `{file://}` path that is not there are two such values. The value of such a
// field is unknown, thus the command stops and names the field.
//
// A guess is not safe here. The encryption field of a cluster that encrypts the secrets of etcd
// holds a `{file://}` reference to the keys of that cluster. A reference that furyctl cannot follow
// reads as an empty field, thus as a cluster with no encryption at rest, and furyctl writes a new
// key. The API server then reads the secrets of etcd with a key that did not write
// them, and those secrets stay unreadable.
//
// The answer covers the fields that Select and Encryption read.
func (c *Config) Unresolved() []string {
	return lo.Uniq(c.unresolved)
}

// forget drops the fields that furyctl could not read. The caller reads again what it needs the
// answer about: see Unreadable.
func (c *Config) forget() {
	c.unresolved = nil
}

// readable reports whether furyctl could read the value of the field. Read the field first: the
// answer comes from the reads that came before.
//
// An empty answer from String is not enough to tell an empty field from one that furyctl cannot
// read. Two kinds of value give an empty answer: a dynamic value that furyctl cannot resolve, which
// expand gives back as it is, and a value that is not text, which String reports here.
func (c *Config) readable(path string) bool {
	return !slices.Contains(c.unresolved, path)
}

// expand resolves a dynamic value, for example `{env://AUTH_TYPE}` or `{file://./type}`. The
// configuration loader of furyctl resolves such a value too, thus a configuration file that holds
// one gets the same secrets as a file that holds the value itself.
//
// A value that furyctl cannot read keeps the field in Unresolved, and comes back as it is. An empty
// string makes the field look absent, and an absent field is an answer: the encryption field, for
// example, is empty for a cluster that holds no encryption at rest.
//
// A value that resolves to nothing is such a value. The parser reports an error for an environment
// variable with no value, and no error for a file with no value in it, thus this check makes the two
// the same. A file with no value in it is a file that furyctl created and could not write: see
// writeNew.
func (c *Config) expand(path, value string) string {
	if !strings.Contains(value, "://") {
		return value
	}

	resolved, err := parserx.NewConfigParser(c.dir).ParseDynamicValue(value)
	if err != nil {
		c.unresolved = append(c.unresolved, path)

		return value
	}

	text, ok := resolved.(string)
	if !ok || strings.TrimSpace(text) == "" {
		c.unresolved = append(c.unresolved, path)

		return value
	}

	return text
}

// value walks the fields of the path and returns the value of the last one.
func (c *Config) value(path string) any {
	if c == nil {
		return nil
	}

	var current any = c.root

	for field := range strings.SplitSeq(path, ".") {
		parent, ok := current.(map[string]any)
		if !ok {
			return nil
		}

		current, ok = parent[field]
		if !ok {
			return nil
		}
	}

	return current
}
