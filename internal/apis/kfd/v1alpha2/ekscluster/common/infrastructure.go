// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"path"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
)

type Infrastructure struct {
	*cluster.OperationPhase

	FuryctlConf private.EksclusterKfdV1Alpha2
	ConfigPath  string
	DistroPath  string
}

func (i *Infrastructure) Prepare() error {
	if err := i.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating infrastructure folder: %w", err)
	}

	if err := i.copyFromTemplate(); err != nil {
		return err
	}

	if err := i.CreateTerraformFolderStructure(); err != nil {
		return fmt.Errorf("error creating infrastructure folder structure: %w", err)
	}

	return nil
}

func (i *Infrastructure) copyFromTemplate() error {
	furyctlMerger, err := i.CreateFuryctlMerger(
		i.DistroPath,
		i.ConfigPath,
		"kfd-v1alpha2",
		"ekscluster",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	vpcInstallerPath := path.Join(i.Path, "..", "vendor", "installers", "eks", "modules", "vpc")
	vpnInstallerPath := path.Join(i.Path, "..", "vendor", "installers", "eks", "modules", "vpn")

	mCfg.Data["infrastructure"] = map[any]any{
		"vpcInstallerPath": vpcInstallerPath,
		"vpnInstallerPath": vpnInstallerPath,
	}

	err = i.CopyFromTemplate(
		mCfg,
		"infrastructure",
		path.Join(
			i.DistroPath,
			"templates",
			cluster.OperationPhaseInfrastructure,
			"ekscluster",
			"terraform",
		),
		path.Join(i.Path, "terraform"),
		i.ConfigPath,
	)
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	return nil
}
