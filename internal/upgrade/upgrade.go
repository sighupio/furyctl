// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgrade

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Upgrade struct {
	paths cluster.CreatorPaths
	kind  string

	Enabled bool
	From    string
	To      string
}

func (u *Upgrade) Exec(phase string) error {
	if !u.Enabled {
		return nil
	}

	logrus.Infof(
		"Running %s upgrade from %s to %s...",
		phase,
		u.From,
		u.To,
	)

	from := semver.EnsureNoPrefix(u.From)
	to := semver.EnsureNoPrefix(u.To)

	// Compose the path to the upgrade script.
	upgradePath := path.Join(
		u.paths.WorkDir,
		"upgrades",
		fmt.Sprintf("%s-%s", from, to),
		strings.ToLower(u.kind),
	)

	upgradeScript := path.Join(upgradePath, fmt.Sprintf("%s.sh", phase))

	if _, err := os.Stat(upgradeScript); err != nil {
		if os.IsNotExist(err) {
			logrus.Debug("Upgrade script not found, skipping...")

			return nil
		}

		return fmt.Errorf("error checking upgrade path: %w", err)
	}

	shellRunner := shell.NewRunner(
		execx.NewStdExecutor(),
		shell.Paths{
			Shell:   "sh",
			WorkDir: upgradePath,
		},
	)

	if _, err := shellRunner.Run(upgradeScript); err != nil {
		return fmt.Errorf("error running upgrade script: %w", err)
	}

	return nil
}

func New(
	paths cluster.CreatorPaths,
	kind string,
) *Upgrade {
	return &Upgrade{
		paths: paths,
		kind:  kind,
	}
}
