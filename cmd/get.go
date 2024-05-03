// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/get"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewGetCommand(tracker *analytics.Tracker) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource (e.g. kubeconfig) from a cluster",
	}

	getCmd.AddCommand(get.NewKubeconfigCmd(tracker))

	return getCmd
}
