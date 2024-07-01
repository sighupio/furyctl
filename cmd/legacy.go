// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/legacy"
)

var legacyCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:   "legacy",
	Short: "Legacy commands for compatibility with older versions of furyctl",
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	legacyCmd.AddCommand(legacy.VendorCmd)
}
