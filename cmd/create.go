// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/create"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewCreateCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cluster or a sample configuration file",
	}

	configCmd, err := create.NewConfigCommand(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating config command: %w", err)
	}

	createCmd.AddCommand(configCmd)

	// Configure create cluster command as alias of apply command.
	applyCmd, err := NewApplyCommand(tracker)
	if err != nil {
		return nil, err
	}

	clusterCmd := &cobra.Command{
		Use:    "cluster",
		Short:  "Apply the configuration to create or upgrade a battle-tested Kubernetes Fury cluster",
		PreRun: applyCmd.PreRun,
		RunE:   applyCmd.RunE,
	}

	clusterCmd.Flags().AddFlagSet(applyCmd.Flags())

	createCmd.AddCommand(clusterCmd)

	return createCmd, nil
}
