// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/validate"
)

func NewValidateCommand(furyctlBinVersion string) *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate fury config files and dependencies",
	}

	validateCmd.AddCommand(validate.NewConfigCmd(furyctlBinVersion))
	validateCmd.AddCommand(validate.NewDependenciesCmd(furyctlBinVersion))

	return validateCmd
}
