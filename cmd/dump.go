// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/dump"
)

func NewDumpCmd() *cobra.Command {
	dumpCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump rendered templates or other useful objects to the filesystem",
	}

	dumpCmd.AddCommand(dump.NewTemplateCmd())
	dumpCmd.AddCommand(dump.NewDumpCLIReferenceCmd())

	return dumpCmd
}
