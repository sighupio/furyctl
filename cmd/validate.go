// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/validate"
)

var validateCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:   "validate",
	Short: "Validate a configuration file and the dependencies relative to the Kubernetes Fury Distribution version specified in it",
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	validateCmd.AddCommand(validate.ConfigCmd)
	validateCmd.AddCommand(validate.DependenciesCmd)
}
