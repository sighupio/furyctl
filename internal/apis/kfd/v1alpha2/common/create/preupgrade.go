// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/upgrade"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/reducers"
	"github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	errUpgradeCanceled          = errors.New("upgrade canceled by user")
	errUpgradeFlagNotSet        = errors.New("upgrade flag not set by user")
	errUpgradePathNotFound      = errors.New("upgrade path not found")
	errGettingDistroVersionFrom = errors.New("error while getting distribution version from")
	errGettingDistroVersionTo   = errors.New("error while getting distribution version to")
)

type PreUpgrade struct {
	*cluster.OperationPhase
	dryRun               bool
	kind                 string
	upgrade              *upgrade.Upgrade
	upgradeFlag          bool
	reducers             reducers.Reducers
	diffs                diff.Changelog
	forceFlag            []string
	paths                cluster.CreatorPaths
	externalUpgradesPath string
	skipNodesUpgrade     bool
}

//nolint:revive // ignore arguments limit
func NewPreUpgrade(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
	upgradeFlag bool,
	forceFlag []string,
	upgr *upgrade.Upgrade,
	reducers reducers.Reducers,
	diffs diff.Changelog,
	externalUpgradesPath string,
	skipNodesUpgrade bool,
) *PreUpgrade {
	phaseOp := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, "upgrades"),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &PreUpgrade{
		OperationPhase:       phaseOp,
		dryRun:               dryRun,
		kind:                 kind,
		upgrade:              upgr,
		upgradeFlag:          upgradeFlag,
		reducers:             reducers,
		diffs:                diffs,
		forceFlag:            forceFlag,
		paths:                paths,
		externalUpgradesPath: externalUpgradesPath,
		skipNodesUpgrade:     skipNodesUpgrade,
	}
}

func (p *PreUpgrade) Exec() error {
	var ok bool

	logrus.Info("Running preupgrade phase...")

	logrus.Debug("Cleaning up upgrade folder...")

	if err := iox.CheckDirIsEmpty(p.Path); err != nil {
		if err := os.RemoveAll(p.Path); err != nil {
			return fmt.Errorf("error while cleaning vendor folder: %w", err)
		}
	}

	if err := p.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating preupgrade phase folder: %w", err)
	}

	furyctlMerger, err := p.CreateFuryctlMerger(
		p.paths.DistroPath,
		p.paths.ConfigPath,
		"kfd-v1alpha2",
		strings.ToLower(p.kind),
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["upgrade"] = map[any]any{
		"skipNodesUpgrade": p.skipNodesUpgrade,
	}

	p.CopyPathsToConfig(&mCfg)

	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-preupgrade-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("Preupgrade configuration file path %s", confPath)

	if err = os.WriteFile(confPath, outYaml, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	var upgradesPath string

	if p.externalUpgradesPath == "" {
		subFS, err := fs.Sub(configs.Tpl, path.Join("upgrades", strings.ToLower(p.kind)))
		if err != nil {
			return fmt.Errorf("error getting subfs: %w", err)
		}

		tmpUpgradesFolder, err := os.MkdirTemp("", "furyctl-create-preupgrade-")
		if err != nil {
			return fmt.Errorf("error creating temp folder: %w", err)
		}

		if err := iox.CopyRecursive(subFS, tmpUpgradesFolder); err != nil {
			return fmt.Errorf("error copying template files: %w", err)
		}

		defer os.RemoveAll(tmpUpgradesFolder)

		upgradesPath = tmpUpgradesFolder
	} else {
		upgradesPath = path.Join(p.externalUpgradesPath, strings.ToLower(p.kind))
	}

	templateModel, err := template.NewTemplateModel(
		upgradesPath,
		p.Path,
		confPath,
		outDirPath1,
		p.paths.ConfigPath,
		".tpl",
		false,
		p.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	if err := templateModel.Generate(); err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	distributionVersionChanges := p.diffs.Filter([]string{"spec", "distributionVersion"})
	if len(distributionVersionChanges) > 0 {
		distributionVersionChange := distributionVersionChanges[0]

		p.upgrade.From, ok = distributionVersionChange.From.(string)
		if !ok {
			return errGettingDistroVersionFrom
		}

		p.upgrade.To, ok = distributionVersionChange.To.(string)
		if !ok {
			return errGettingDistroVersionTo
		}

		logrus.Warnf(
			"Distribution version changed from %s to %s, you are about to upgrade the cluster.\n",
			p.upgrade.From,
			p.upgrade.To,
		)

		if !p.upgradeFlag {
			return errUpgradeFlagNotSet
		}

		from := semver.EnsureNoPrefix(p.upgrade.From)
		to := semver.EnsureNoPrefix(p.upgrade.To)

		upgradePath := path.Join(p.Path, fmt.Sprintf("%s-%s", from, to))

		if _, err := os.Stat(upgradePath); err != nil {
			if cluster.IsForceEnabledForFeature(p.forceFlag, cluster.ForceFeatureUpgrades) {
				logrus.Warn("An upgrade path was not found, but the force flag was set, so the process will continue.")

				p.upgrade.Enabled = false

				return nil
			}

			if os.IsNotExist(err) {
				return fmt.Errorf("%w: unable to upgrade from %s to %s, please check the available upgrade "+
					"paths with the command 'furyctl get upgrade-paths'",
					errUpgradePathNotFound, p.upgrade.From, p.upgrade.To)
			}

			return fmt.Errorf("error checking upgrade path: %w", err)
		}

		// We should find a smarer way to stop the process if the reducers are from the upgrade or not
		// if len(p.reducers) > 0 {
		// return errUpgradeWithReducersNotAllowed
		// }.

		if !cluster.IsForceEnabledForFeature(p.forceFlag, cluster.ForceFeatureUpgrades) {
			_, err := fmt.Println("\nAre you sure you want to continue? Only 'yes' will be accepted to confirm.")
			if err != nil {
				return fmt.Errorf("error writing to stdout: %w", err)
			}

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

	logrus.Info("Preupgrade phase completed successfully")

	return nil
}
