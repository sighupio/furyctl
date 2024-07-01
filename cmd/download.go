// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/download"
)

var downloadCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:   "download",
	Short: "Download all dependencies for the Kubernetes Fury Distribution version specified in the configuration file",
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	downloadCmd.AddCommand(download.DependenciesCmd)
}
