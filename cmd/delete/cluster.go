// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/cluster"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	ErrYamlUnmarshalFile = errors.New("error unmarshaling yaml file")
	ErrDebugFlagNotSet   = errors.New("debug flag not set")
	ErrFuryctlFlagNotSet = errors.New("furyctl flag not set")
	ErrForceFlagNotSet   = errors.New("force flag not set")
)

func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Deletes a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			debug, ok := cobrax.Flag[bool](cmd, "debug").(bool)
			if !ok {
				return ErrDebugFlagNotSet
			}

			furyctlPath, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				return ErrFuryctlFlagNotSet
			}

			force, ok := cobrax.Flag[bool](cmd, "force").(bool)
			if !ok {
				return ErrForceFlagNotSet
			}

			execx.Debug = debug

			minimalConf, err := yamlx.FromFileV3[config.Furyctl](furyctlPath)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrYamlUnmarshalFile, err)
			}

			clusterDeleter, err := cluster.NewDeleter(minimalConf, force)
			if err != nil {
				return fmt.Errorf("error while initializing cluster deleter: %w", err)
			}

			err = clusterDeleter.Delete()
			if err != nil {
				return fmt.Errorf("error while deleting cluster: %w", err)
			}

			_, err = fmt.Println("cluster deleted")
			if err != nil {
				return fmt.Errorf("error while printing success message: %w", err)
			}

			return nil
		},
	}

	return cmd
}
