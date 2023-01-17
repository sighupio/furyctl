// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/download"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewDownloadCmd(tracker *analytics.Tracker) *cobra.Command {
	dumpCmd := &cobra.Command{
		Use:   "download",
		Short: "Download all dependencies from the Kubernetes Fury Distribution specified in the config file",
	}

	dumpCmd.AddCommand(download.NewDependenciesCmd(tracker))

	return dumpCmd
}
