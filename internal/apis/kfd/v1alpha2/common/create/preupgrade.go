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

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/upgrade"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errUpgradeCanceled               = errors.New("upgrade canceled by user")
	errUpgradeFlagNotSet             = errors.New("upgrade flag not set by user")
	errUpgradeWithReducersNotAllowed = errors.New("upgrade with reducers not allowed")
	errUpgradePathNotFound           = errors.New("upgrade path not found")
	errGettingDistoVersionFrom       = errors.New("error while getting distribution version from")
	errGettingDistroVersionTo        = errors.New("error while getting distribution version to")
)

type PreUpgrade struct {
	*cluster.OperationPhase
	distroPath      string
	furyctlConfPath string
	dryRun          bool
	kind            string
	upgrade         *upgrade.Upgrade
	upgradeFlag     bool
	reducers        v1alpha2.Reducers
	merger          v1alpha2.Merger
	diffs           diff.Changelog
	forceFlag       bool
}

func NewPreUpgrade(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
	upgradeFlag bool,
	forceFlag bool,
	upgr *upgrade.Upgrade,
	reducers v1alpha2.Reducers,
	diffs diff.Changelog,
) *PreUpgrade {
	phaseOp := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, "upgrades"),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &PreUpgrade{
		OperationPhase:  phaseOp,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		dryRun:          dryRun,
		kind:            kind,
		upgrade:         upgr,
		upgradeFlag:     upgradeFlag,
		reducers:        reducers,
		merger:          v1alpha2.NewBaseMerger(paths.DistroPath, kind, paths.ConfigPath),
		diffs:           diffs,
		forceFlag:       forceFlag,
	}
}

func (p *PreUpgrade) Exec() error {
	var ok bool

	logrus.Info("Running preupgrade phase...")

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating preupgrade phase folder: %w", err)
	}

	furyctlMerger, err := p.merger.Create()
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
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

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(p.distroPath, "templates", "upgrades", strings.ToLower(p.kind)),
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

	if err := templateModel.Generate(); err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	distributionVersionChanges := p.diffs.Filter([]string{"spec", "distributionVersion"})
	if len(distributionVersionChanges) > 0 {
		distributionVersionChange := distributionVersionChanges[0]

		p.upgrade.From, ok = distributionVersionChange.From.(string)
		if !ok {
			return errGettingDistoVersionFrom
		}

		p.upgrade.To, ok = distributionVersionChange.To.(string)
		if !ok {
			return errGettingDistroVersionTo
		}

		fmt.Printf(
			"WARNING: Distribution version changed from %s to %s, you are about to upgrade the cluster.\n",
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
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: unable to upgrade from %s to %s", errUpgradePathNotFound, p.upgrade.From, p.upgrade.To)
			}

			return fmt.Errorf("error checking upgrade path: %w", err)
		}

		if len(p.reducers) > 0 {
			return errUpgradeWithReducersNotAllowed
		}

		if !p.forceFlag {
			if _, err := fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm."); err != nil {
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
