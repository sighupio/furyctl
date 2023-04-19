// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/sighupio/furyctl/internal/analytics"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

func NewVersionCmd(versions map[string]string, tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of furyctl",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		Run: func(_ *cobra.Command, _ []string) {
			keys := maps.Keys(versions)

			slices.Sort(keys)

			for _, k := range keys {
				fmt.Printf("%s: %s\n", k, versions[k])
			}

			cmdEvent.AddSuccessMessage("version command executed successfully")
			tracker.Track(cmdEvent)
		},
	}
}
