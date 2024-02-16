// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"bufio"
	"fmt"
	"os"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	ForceFeatureAll              string = "all"
	ForceFeatureMigrations       string = "migrations"
	ForceFeatureUpgrades         string = "upgrades"
	ForceFeaturePodsRunningCheck string = "pods-running-check"
)

func IsForceEnabledForFeature(force []string, feature string) bool {
	for _, f := range force {
		if f == "all" || f == feature {
			return true
		}
	}

	return false
}

//nolint:revive // force bool needs to be here
func AskConfirmation(force bool) (bool, error) {
	if !force {
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
