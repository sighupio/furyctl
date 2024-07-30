// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/sighupio/furyctl/cmd/renew"
	"github.com/spf13/cobra"
)

func NewRenewCmd() *cobra.Command {
	renewCmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew a resource (e.g. certificates) of a cluster",
	}

	renewCmd.AddCommand(renew.NewCertificatesCmd())

	return renewCmd
}
