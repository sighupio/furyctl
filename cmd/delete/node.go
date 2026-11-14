// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/airgap"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/flags"
)

func NewNodeCmd() *cobra.Command {
	nodeCmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   "node <hostname>",
		Short: "Deletes one worker of an Immutable cluster",
		Long: "Deletes one worker of an Immutable cluster: drains it, deletes its Node object, erases the partition " +
			"table of its install disk and restarts it. The host boots from the network and waits there.\n\n" +
			"Then remove the host from the configuration file and run furyctl apply. " +
			"If the host stays in the configuration file, the next furyctl apply installs it again.",
		PreRun: func(cmd *cobra.Command, _ []string) {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			// Load and merge flags from configuration file.
			if err := flags.LoadAndMergeCommandFlags("delete"); err != nil {
				logrus.Fatalf("%v", err)
			}
		},
		RunE: func(_ *cobra.Command, args []string) error {
			// Air-gapped: extract --airgap-bundle (if set) and rewire to run offline before reading flags.
			if err := airgap.MaybePrepare(); err != nil {
				return fmt.Errorf("error preparing air-gapped bundle: %w", err)
			}

			flags, err := getDeleteClusterCmdFlags()
			if err != nil {
				return err
			}

			// A phase from the flags of the configuration file does not apply to one node.
			flags.Phase = cluster.OperationPhaseAll
			flags.DeleteNode = args[0]

			return runDelete(flags)
		},
	}

	nodeCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	nodeCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	nodeCmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	nodeCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are downloaded",
	)

	nodeCmd.Flags().Bool(
		"dry-run",
		false,
		"When set furyctl only checks the host: it asks for no confirmation and changes nothing",
	)

	nodeCmd.Flags().Bool(
		"force",
		false,
		"WARNING: furyctl won't ask for confirmation and will delete the node.",
	)

	nodeCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the distribution modules, installers and binaries",
	)

	airgap.RegisterFlags(nodeCmd)

	nodeCmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	return nodeCmd
}
