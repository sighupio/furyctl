// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"path"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kapp"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

// NewDistributionPhase builds the distribution delete with the real kapp, and the networking type of the merged
// config: the furyctl file alone can leave it to the distribution defaults.
func NewDistributionPhase(paths cluster.DeleterPaths, kfdManifest config.KFD, dryRun bool) (*Distribution, error) {
	phase := cluster.NewOperationPhase(path.Join(paths.WorkDir, cluster.OperationPhaseDistribution),
		kfdManifest.Tools, paths.BinPath)

	if err := phase.CreateRootFolder(); err != nil {
		return nil, fmt.Errorf("error creating distribution phase folder: %w", err)
	}

	cfg, err := mergedConfig(phase, paths)
	if err != nil {
		return nil, err
	}

	runner := kapp.NewRunner(execx.NewStdExecutor(), kapp.Paths{Kapp: phase.KappPath, WorkDir: phase.Path}, false)

	return NewDistribution(runner, networkingType(cfg.Data["spec"]), dryRun), nil
}

// networkingType reads spec.distribution.modules.networking.type, empty when it is missing.
func networkingType(spec map[any]any) string {
	node := any(spec)

	for _, key := range []string{"distribution", "modules", "networking", "type"} {
		m, ok := node.(map[any]any)
		if !ok {
			return ""
		}

		node = m[key]
	}

	if node == nil {
		return ""
	}

	return fmt.Sprint(node)
}

// mergedConfig merges the distribution defaults of the Immutable kind under the furyctl config, as apply does.
func mergedConfig(phase *cluster.OperationPhase, paths cluster.DeleterPaths) (templatex.Config, error) {
	merger, err := phase.CreateFuryctlMerger(paths.DistroPath, paths.ConfigPath, "kfd-v1alpha2", "immutable")
	if err != nil {
		return templatex.Config{}, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	cfg, err := templatex.NewConfigWithoutData(merger, []string{})
	if err != nil {
		return templatex.Config{}, fmt.Errorf("error creating template config: %w", err)
	}

	return cfg, nil
}
