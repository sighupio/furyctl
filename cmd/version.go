// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewVersionCmd(versions map[string]string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of furyctl",
		Run: func(_ *cobra.Command, _ []string) {
			for k, v := range versions {
				fmt.Printf("%s: %s\n", k, v)
			}
		},
	}
}
