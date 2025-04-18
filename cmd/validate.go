// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/validate"
)

func NewValidateCmd() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a configuration file and the dependencies relative to the SIGHUP Distribution version specified in it",
	}

	validateCmd.AddCommand(validate.NewConfigCmd())
	validateCmd.AddCommand(validate.NewDependenciesCmd())

	return validateCmd
}
