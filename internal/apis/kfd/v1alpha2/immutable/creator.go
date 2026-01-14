// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/pkg/template"
)

const (
	InfrastructurePhaseSchemaPath = ".spec.infrastructure"
	KubernetesPhaseSchemaPath     = ".spec.kubernetes"
	DistributionPhaseSchemaPath   = ".spec.distribution"
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

	// Render merged configuration (defaults + user config).
	mergedConfig, err := c.RenderConfig()
	if err != nil {
		return fmt.Errorf("failed to render config: %w", err)
	}

	// Create infrastructure phase.
	infraPath := filepath.Join(c.paths.WorkDir, "infrastructure")

	phase := cluster.NewOperationPhase(
		infraPath,
		c.kfdManifest.Tools,
		c.paths.BinPath,
	)

	infra := create.NewInfrastructure(phase, c.paths.ConfigPath, mergedConfig, c.paths.DistroPath)

	// Execute phase.
	if err := infra.Exec(); err != nil {
		return fmt.Errorf("infrastructure phase failed: %w", err)
	}

	logrus.Info("Infrastructure phase completed successfully")

	return nil
}

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	schemaPath, ok := supported.GetSchemaPath(phase)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}

	return schemaPath, nil
}

// RenderConfig loads the complete furyctl configuration merged with defaults from fury-distribution.
// For infrastructure phase, we need the full spec including infrastructure config.
func (c *ClusterCreator) RenderConfig() (map[string]any, error) {
	// Create phase for infrastructure.
	phase := cluster.NewOperationPhase(
		path.Join(c.paths.WorkDir, cluster.OperationPhaseInfrastructure),
		c.kfdManifest.Tools,
		c.paths.BinPath,
	)

	// Use CreateFuryctlMerger to merge defaults + user config.
	furyctlMerger, err := phase.CreateFuryctlMerger(
		c.paths.DistroPath,
		c.paths.ConfigPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating furyctl merger: %w", err)
	}

	// Create template config without data.
	tfCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error while creating template config: %w", err)
	}

	// tfCfg.Data already contains the properly structured merged config
	// with "spec", "metadata", etc. from the user config merged with defaults.
	// Convert to map[string]any (including nested maps).
	result := make(map[string]any)

	for k, v := range tfCfg.Data {
		result[k] = convertValue(v)
	}

	return result, nil
}

// convertValue recursively converts any value, handling maps and slices.
func convertValue(v any) any {
	switch val := v.(type) {
	case map[any]any:
		// Convert map[any]any to map[string]any.
		result := make(map[string]any)

		for k, v := range val {
			keyStr, ok := k.(string)
			if !ok {
				continue
			}

			result[keyStr] = convertValue(v)
		}

		return result
	case map[string]any:
		// Already correct type, but check nested values.
		result := make(map[string]any)
		for k, v := range val {
			result[k] = convertValue(v)
		}

		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = convertValue(item)
		}

		return result
	default:
		return val
	}
}
