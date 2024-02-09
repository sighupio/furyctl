// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/cluster"
)

const (
	KubernetesPhaseSchemaPath   = ".spec.kubernetes"
	DistributionPhaseSchemaPath = ".spec.distribution"
	PluginsPhaseSchemaPath      = ".spec.plugins"
	AllPhaseSchemaPath          = ""
)

func NewSchemaSettings() *SchemaSettings {
	return &SchemaSettings{}
}

type SchemaSettings struct{}

func (*SchemaSettings) SchemaPathForPhase(phase string) (string, error) {
	switch phase {
	case cluster.OperationPhaseKubernetes:
		return KubernetesPhaseSchemaPath, nil

	case cluster.OperationPhaseDistribution:
		return DistributionPhaseSchemaPath, nil

	case cluster.OperationPhasePlugins:
		return PluginsPhaseSchemaPath, nil

	case cluster.OperationPhaseAll:
		return AllPhaseSchemaPath, nil

	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}
}

type ExtraSchemaValidator struct{}

func (*ExtraSchemaValidator) Validate(_ string) error {
	return nil
}
