// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/cluster"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	InfrastructurePhaseSchemaPath = ".spec.infrastructure"
)

var (
	ErrUnsupportedPhase              = errors.New("unsupported phase")
	ErrClusterCreationNotImplemented = errors.New("cluster creation not implemented for Immutable kind")
)

type ClusterCreator struct {
	paths       cluster.CreatorPaths
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
}

func (c *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}
}

func (c *ClusterCreator) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CreatorPropertyConfigPath:
		if s, ok := value.(string); ok {
			c.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyDistroPath:
		if s, ok := value.(string); ok {
			c.paths.DistroPath = s
		}

	case cluster.CreatorPropertyWorkDir:
		if s, ok := value.(string); ok {
			c.paths.WorkDir = s
		}

	case cluster.CreatorPropertyBinPath:
		if s, ok := value.(string); ok {
			c.paths.BinPath = s
		}

	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(public.ImmutableKfdV1Alpha2); ok {
			c.furyctlConf = s
		}

	case cluster.CreatorPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			c.kfdManifest = s
		}

	case cluster.CreatorPropertyPhase:
		if s, ok := value.(string); ok {
			c.phase = s
		}
	}
}

func (c *ClusterCreator) Create(_ string, _, _ int) error {
	if c.phase == "" {
		return ErrClusterCreationNotImplemented
	}

	switch c.phase {
	case cluster.OperationPhaseInfrastructure:
		return c.createInfrastructure()

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPhase, c.phase)
	}
}

func (c *ClusterCreator) createInfrastructure() error {
	logrus.Info("Creating infrastructure phase...")

	// Parse config to map[string]any.
	configData, err := yamlx.FromFileV3[map[string]any](c.paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Create infrastructure phase.
	infraPath := filepath.Join(c.paths.WorkDir, ".furyctl", "infrastructure")

	phase := &cluster.OperationPhase{
		Path: infraPath,
	}

	infra := create.NewInfrastructure(phase, c.paths.ConfigPath, configData)

	// Execute phase.
	if err := infra.Exec(); err != nil {
		return fmt.Errorf("infrastructure phase failed: %w", err)
	}

	logrus.Info("Infrastructure phase completed successfully")

	return nil
}

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	switch phase {
	case cluster.OperationPhaseInfrastructure:
		return InfrastructurePhaseSchemaPath, nil

	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}
}
