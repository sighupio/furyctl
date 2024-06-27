// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/download"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewDownloadCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	dumpCmd := &cobra.Command{
		Use:   "download",
		Short: "Download all dependencies for the Kubernetes Fury Distribution version specified in the configuration file",
	}

	dependenciesCmd, err := download.NewDependenciesCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating dependencies command: %w", err)
	}

	dumpCmd.AddCommand(dependenciesCmd)

	return dumpCmd, nil
}
