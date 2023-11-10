// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/upgrade"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errUpgradeCanceled               = errors.New("upgrade canceled by user")
	errUpgradeFlagNotSet             = errors.New("upgrade flag not set by user")
	errUpgradeWithReducersNotAllowed = errors.New("upgrade with reducers not allowed")
)

type PreUpgrade struct {
	*cluster.OperationPhase
	distroPath      string
	furyctlConfPath string
	dryRun          bool
	kubeconfig      string
	kind            string
	upgrade         *upgrade.Upgrade
	upgradeFlag     bool
	reducers        v1alpha2.Reducers
	status          *create.Status
	forceFlag       bool
}

func NewPreUpgrade(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
	kubeconfig string,
	upgradeFlag bool,
	forceFlag bool,
	upgr *upgrade.Upgrade,
	reducers v1alpha2.Reducers,
	status *create.Status,
) (*PreUpgrade, error) {
	phaseOp, err := cluster.NewOperationPhase(path.Join(paths.WorkDir, "upgrades"), kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating preupgrade phase: %w", err)
	}

	return &PreUpgrade{
		OperationPhase:  phaseOp,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		dryRun:          dryRun,
		kubeconfig:      kubeconfig,
		kind:            kind,
		upgrade:         upgr,
		upgradeFlag:     upgradeFlag,
		reducers:        reducers,
		status:          status,
		forceFlag:       forceFlag,
	}, nil
}

func (p *PreUpgrade) Exec() error {
	logrus.Info("Running preupgrade phase...")

	distributionVersionChanges := p.status.Diffs.Filter([]string{"spec", "distributionVersion"})
	if len(distributionVersionChanges) > 0 {
		if len(p.reducers) > 0 {
			return errUpgradeWithReducersNotAllowed
		}

		distributionVersionChange := distributionVersionChanges[0]

		p.upgrade.From = distributionVersionChange.From.(string)
		p.upgrade.To = distributionVersionChange.To.(string)

		fmt.Printf(
			"WARNING: Distribution version changed from %s to %s, you are about to upgrade the cluster.\n",
			p.upgrade.From,
			p.upgrade.To,
		)

		if !p.upgradeFlag {
			return errUpgradeFlagNotSet
		}

		if !p.forceFlag {
			fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm.")

			prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))

			prompt, err := prompter.Ask("yes")
			if err != nil {
				return fmt.Errorf("error reading user input: %w", err)
			}

			if !prompt {
				return errUpgradeCanceled
			}
		}

		p.upgrade.Enabled = true
	}

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating preupgrade phase folder: %w", err)
	}

	furyctlMerger, err := p.createFuryctlMerger()
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["paths"] = map[any]any{
		"helm":       p.HelmPath,
		"kustomize":  p.KustomizePath,
		"kubeconfig": p.kubeconfig,
		"kubectl":    p.KubectlPath,
		"yq":         p.YqPath,
		"terraform":  p.TerraformPath,
	}

	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-preupgrade-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(p.distroPath, "templates", "upgrades"),
		path.Join(p.Path),
		confPath,
		outDirPath1,
		p.furyctlConfPath,
		".tpl",
		false,
		p.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	upgradePath := path.Join(
		p.Path,
		fmt.Sprintf("%s-%s", p.upgrade.From, p.upgrade.To),
		strings.ToLower(p.kind),
	)

	if _, err := os.Stat(upgradePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("unable to upgrade from %s to %s, upgrade path not found", p.upgrade.From, p.upgrade.To)
		}

		return fmt.Errorf("error checking upgrade path: %w", err)
	}

	logrus.Info("Preupgrade phase completed successfully")

	return nil
}

func (p *PreUpgrade) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(p.distroPath, "defaults", fmt.Sprintf("%s-kfd-v1alpha2.yaml", strings.ToLower(p.kind)))

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](p.furyctlConfPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", p.furyctlConfPath, err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(furyctlConf, ".spec.distribution"),
	)

	_, err = merger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	return reverseMerger, nil
}
