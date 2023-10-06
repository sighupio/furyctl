// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/dump"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewDumpCommand(tracker *analytics.Tracker) *cobra.Command {
	dumpCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump manifests templates and other useful KFD objects",
	}

	dumpCmd.AddCommand(dump.NewTemplateCmd(tracker))

	return dumpCmd
}
