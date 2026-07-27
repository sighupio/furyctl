// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"bufio"
	"fmt"
	"os"
	"slices"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	ForceFeatureAll              string = "all"
	ForceFeatureMigrations       string = "migrations"
	ForceFeatureUpgrades         string = "upgrades"
	ForceFeaturePodsRunningCheck string = "pods-running-check"
)

func IsForceEnabledForFeature(force []string, feature string) bool {
	return slices.ContainsFunc(force, func(f string) bool {
		return f == "all" || f == feature
	})
}

//nolint:revive // force bool needs to be here
func AskConfirmationWithMessage(force bool, msg string) (bool, error) {
	if !force {
		if _, err := fmt.Println(msg); err != nil {
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

func AskConfirmation(force bool) (bool, error) {
	return AskConfirmationWithMessage(force, "\nWARNING: You are about to apply changes to the cluster configuration "+
		"that could potentially produce data loss or service disruption.")
}
