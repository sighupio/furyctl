// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/validate"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewValidateCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a configuration file and the dependencies relative to the Kubernetes Fury Distribution version specified in it",
	}

	configCmd, err := validate.NewConfigCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating config command: %w", err)
	}

	dependenciesCmd, err := validate.NewDependenciesCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating dependencies command: %w", err)
	}

	validateCmd.AddCommand(configCmd)
	validateCmd.AddCommand(dependenciesCmd)

	return validateCmd, nil
}
