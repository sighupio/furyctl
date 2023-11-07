// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/create"
	distrorules "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/rules"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	DistributionPhaseSchemaPath = ".spec.distribution"
	PluginsPhaseSchemaPath      = ".spec.plugins"
	AllPhaseSchemaPath          = ""
)

var (
	ErrUnsupportedPhase = errors.New("unsupported phase")
	ErrAbortedByUser    = errors.New("operation aborted by user")
)

type ClusterCreator struct {
	paths       cluster.CreatorPaths
	furyctlConf public.KfddistributionKfdV1Alpha2
	stateStore  state.Storer
	kfdManifest config.KFD
	phase       string
	dryRun      bool
	force       bool
	upgrade     bool
}

func (c *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}

	c.stateStore = state.NewStore(
		c.paths.DistroPath,
		c.paths.ConfigPath,
		c.paths.Kubeconfig,
		c.paths.WorkDir,
		c.kfdManifest.Tools.Common.Kubectl.Version,
		c.paths.BinPath,
	)
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

	case cluster.CreatorPropertyKubeconfig:
		if s, ok := value.(string); ok {
			c.paths.Kubeconfig = s
		}

	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(public.KfddistributionKfdV1Alpha2); ok {
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

	case cluster.CreatorPropertyDryRun:
		if b, ok := value.(bool); ok {
			c.dryRun = b
		}

	case cluster.CreatorPropertyForce:
		if b, ok := value.(bool); ok {
			c.force = b
		}

	case cluster.CreatorPropertyUpgrade:
		if b, ok := value.(bool); ok {
			c.upgrade = b
		}
	}
}

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	switch phase {
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

func (c *ClusterCreator) Create(skipPhase string, _ int) error {
	upgr := upgrade.New(c.paths, string(c.furyctlConf.Kind))

	distributionPhase, err := create.NewDistribution(
		c.paths,
		c.furyctlConf,
		c.kfdManifest,
		c.dryRun,
		c.paths.Kubeconfig,
		upgr,
	)
	if err != nil {
		return fmt.Errorf("error while initiating distribution phase: %w", err)
	}

	pluginsPhase, err := commcreate.NewPlugins(
		c.paths,
		c.kfdManifest,
		string(c.furyctlConf.Kind),
		c.dryRun,
		c.paths.Kubeconfig,
	)
	if err != nil {
		return fmt.Errorf("error while initiating plugins phase: %w", err)
	}

	preflight, err := create.NewPreFlight(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
		c.paths.Kubeconfig,
		c.stateStore,
		c.upgrade,
		upgr,
	)
	if err != nil {
		return fmt.Errorf("error while initiating preflight phase: %w", err)
	}

	status, err := preflight.Exec()
	if err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	r, err := distrorules.NewDistroClusterRulesExtractor(c.paths.DistroPath)
	if err != nil {
		if !errors.Is(err, distrorules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}
	}

	switch c.phase {
	case cluster.OperationPhaseDistribution:
		reducers := c.buildReducers(
			status.Diffs,
			r,
			cluster.OperationPhaseDistribution,
		)

		if len(reducers) > 0 {
			confirm, err := c.AskConfirmation()
			if err != nil {
				return fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return ErrAbortedByUser
			}
		}

		if err := distributionPhase.Exec(reducers); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}

	case cluster.OperationPhasePlugins:
		if err := pluginsPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
		}

	case cluster.OperationPhaseAll:
		if skipPhase != cluster.OperationPhaseDistribution {
			reducers := c.buildReducers(
				status.Diffs,
				r,
				cluster.OperationPhaseDistribution,
			)

			if len(reducers) > 0 {
				confirm, err := c.AskConfirmation()
				if err != nil {
					return fmt.Errorf("error while asking for confirmation: %w", err)
				}

				if !confirm {
					return ErrAbortedByUser
				}
			}

			if err := distributionPhase.Exec(reducers); err != nil {
				return fmt.Errorf("error while executing distribution phase: %w", err)
			}
		}

		if skipPhase != cluster.OperationPhasePlugins {
			if err := pluginsPhase.Exec(); err != nil {
				return fmt.Errorf("error while executing plugins phase: %w", err)
			}
		}

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPhase, c.phase)
	}

	if c.dryRun {
		return nil
	}

	if err := c.stateStore.StoreConfig(); err != nil {
		return fmt.Errorf("error while storing cluster config: %w", err)
	}

	if err := c.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while storing distribution config: %w", err)
	}

	return nil
}

func (*ClusterCreator) buildReducers(
	statusDiffs r3diff.Changelog,
	rulesExtractor rules.Extractor,
	phase string,
) v1alpha2.Reducers {
	reducersRules := rulesExtractor.GetReducers(phase)

	filteredReducers := rulesExtractor.ReducerRulesByDiffs(reducersRules, statusDiffs)

	reducers := make(v1alpha2.Reducers, len(filteredReducers))

	if len(filteredReducers) > 0 {
		for _, reducer := range filteredReducers {
			if reducer.Reducers != nil {
				if reducer.Description != nil {
					logrus.Infof("%s", *reducer.Description)
				}

				for _, red := range *reducer.Reducers {
					reducers = append(reducers, v1alpha2.NewBaseReducer(
						red.Key,
						red.From,
						red.To,
						red.Lifecycle,
					),
					)
				}
			}
		}
	}

	return reducers
}

func (c *ClusterCreator) AskConfirmation() (bool, error) {
	if !c.force {
		if _, err := fmt.Println("WARNING: You are about to apply changes to the cluster configuration."); err != nil {
			return false, fmt.Errorf("error while printing to stdout: %w", err)
		}

		if _, err := fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm."); err != nil {
			return false, fmt.Errorf("error while printing to stdout: %w", err)
		}

		prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))

		prompt, err := prompter.Ask("yes")
		if err != nil {
			return false, fmt.Errorf("error reading user input: %w", err)
		}

		if !prompt {
			return false, nil
		}
	}

	return true, nil
}
