// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/dump"
)

var dumpCmd = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	Use:   "dump",
	Short: "Dump manifests templates and other useful KFD objects",
}

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	dumpCmd.AddCommand(dump.TemplateCmd)
}
