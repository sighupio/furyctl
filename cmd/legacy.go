// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/legacy"
)

func NewLegacyCmd() *cobra.Command {
	legacyCmd := &cobra.Command{
		Use:   "legacy",
		Short: "Legacy commands for compatibility with older versions of furyctl",
	}

	legacyCmd.AddCommand(legacy.NewVendorCmd())

	return legacyCmd
}
