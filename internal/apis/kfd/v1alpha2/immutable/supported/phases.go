// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package supported

import (
	"slices"

	"github.com/sighupio/furyctl/internal/cluster"
)

type Phases struct{}

func NewPhases() *Phases {
	return &Phases{}
}

func (*Phases) Get() []string {
	return []string{
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution,
		cluster.OperationPhasePlugins,
	}
}

func (s *Phases) IsSupported(phase string) bool {
	return slices.Contains(s.Get(), phase)
}

// SchemaPaths maps phases to their JSON schema paths in furyctl.yaml.
//
//nolint:gochecknoglobals // Idiomatic configuration map.
var SchemaPaths = map[string]string{
	cluster.OperationPhaseInfrastructure: ".spec.infrastructure",
	cluster.OperationPhaseKubernetes:     ".spec.kubernetes",
	cluster.OperationPhaseDistribution:   ".spec.distribution",
}

// GetSchemaPath returns the schema path for a given phase.
func GetSchemaPath(phase string) (string, bool) {
	path, ok := SchemaPaths[phase]

	return path, ok
}
