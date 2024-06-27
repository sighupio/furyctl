// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/get"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewGetCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource (e.g. kubeconfig) from a cluster",
	}

	kubeconfigCmd, err := get.NewKubeconfigCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig command: %w", err)
	}

	getCmd.AddCommand(kubeconfigCmd)

	return getCmd, nil
}
