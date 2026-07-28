// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package supported

import (
	"github.com/sighupio/furyctl/internal/cluster"
)

func Phases() cluster.SupportedPhases {
	return cluster.SupportedPhases{
		cluster.OperationPhaseInfrastructure,
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution,
		cluster.OperationPhasePlugins,
	}
}

// SchemaPaths maps phases to their JSON schema paths in furyctl.yaml.
//
//nolint:gochecknoglobals // Idiomatic configuration map.
var SchemaPaths = map[string]string{
	cluster.OperationPhaseInfrastructure: ".spec.infrastructure",
	cluster.OperationPhaseKubernetes:     ".spec.kubernetes",
	cluster.OperationPhaseDistribution:   ".spec.distribution",
	cluster.OperationPhasePlugins:        ".spec.plugins",
}

// GetSchemaPath returns the schema path for a given phase.
func GetSchemaPath(phase string) (string, bool) {
	path, ok := SchemaPaths[phase]

	return path, ok
}
