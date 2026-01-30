// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/common"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/upgrade"
)

// Infrastructure wraps the common infrastructure phase.
type Infrastructure struct {
	*common.Infrastructure
	upgrade *upgrade.Upgrade
}

// NewInfrastructure creates a new Infrastructure phase.
func NewInfrastructure(
	phase *cluster.OperationPhase,
	configPath string,
	configData map[string]any,
	distroPath string,
) *Infrastructure {
	return &Infrastructure{
		Infrastructure: &common.Infrastructure{
			OperationPhase: phase,
			ConfigPath:     configPath,
			ConfigData:     configData,
			DistroPath:     distroPath,
		},
	}
}

// Exec executes the infrastructure phase.
func (i *Infrastructure) Exec(_ string, _ *upgrade.State) error {
	if err := i.Prepare(); err != nil {
		return fmt.Errorf("infrastructure phase failed: %w", err)
	}

	return nil
}

// Self returns the operation phase.
func (i *Infrastructure) Self() *cluster.OperationPhase {
	return i.OperationPhase
}

func (i *Infrastructure) SetUpgrade(upgradeEnabled bool) {
	i.upgrade.Enabled = upgradeEnabled
}
