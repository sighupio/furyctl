// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"fmt"
	"path"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
)

var (
	ErrCannotDiffInfrastructurePhase = fmt.Errorf("cannot diff infrastructure phase")
	ErrCannotDiffKubernetesPhase     = fmt.Errorf("cannot diff kubernetes phase")
	ErrCannotDiffDistributionPhase   = fmt.Errorf("cannot diff distribution phase")
)

func NewClusterDiffer(
	shellRunner *shell.Runner,
	kubectlRunner *kubectl.Runner,
) cluster.Differ {
	return &ClusterDiffer{
		shellRunner:   shellRunner,
		kubectlRunner: kubectlRunner,
	}
}

type ClusterDiffer struct {
	shellRunner   *shell.Runner
	kubectlRunner *kubectl.Runner
}

func (*ClusterDiffer) DiffInfrastructurePhase() ([]byte, error) {
	return nil, nil
}

func (*ClusterDiffer) DiffKubernetesPhase() ([]byte, error) {
	return nil, nil
}

func (d *ClusterDiffer) DiffDistributionPhase() ([]byte, error) {
	if _, err := d.shellRunner.Run(path.Join("..", "scripts", "apply.sh")); err != nil {
		return nil, fmt.Errorf("error diffing manifests: %w", err)
	}

	out, err := d.kubectlRunner.Diff("out.yaml")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCannotDiffDistributionPhase, err)
	}

	return out, nil
}
