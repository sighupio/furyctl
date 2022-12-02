// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/configs"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	ErrConfigFlagNotSet  = errors.New("config flag not set")
	ErrVersionFlagNotSet = errors.New("version flag not set")
	ErrKindFlagNotSet    = errors.New("kind flag not set")
	ErrNameFlagNotSet    = errors.New("name flag not set")

	ErrConfigCreationFailed = fmt.Errorf("config creation failed")
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "scaffolds a new furyctl config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				return ErrConfigFlagNotSet
			}
			version, ok := cobrax.Flag[string](cmd, "version").(string)
			if !ok {
				return ErrVersionFlagNotSet
			}
			kind, ok := cobrax.Flag[string](cmd, "kind").(string)
			if !ok {
				return ErrKindFlagNotSet
			}
			name, ok := cobrax.Flag[string](cmd, "name").(string)
			if !ok {
				return ErrNameFlagNotSet
			}

			data, err := configs.Tpl.ReadFile("furyctl.yaml.tpl")
			if err != nil {
				return fmt.Errorf("error reading furyctl yaml template: %w", err)
			}

			tmpl, err := template.New("furyctl.yaml").Parse(string(data))
			if err != nil {
				return fmt.Errorf("error parsing furyctl yaml template: %w", err)
			}

			out, err := createNewEmptyConfigFile(config)
			if err != nil {
				return err
			}

			if err := tmpl.Execute(out, map[string]string{
				"Kind":                kind,
				"Name":                name,
				"DistributionVersion": version,
			}); err != nil {
				return fmt.Errorf("error executing furyctl yaml template: %w", err)
			}

			logrus.Infof("Config file created successfully at: %s", out.Name())

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"version",
		"v",
		"v1.23.3",
		"distribution version to use",
	)

	cmd.Flags().StringP(
		"kind",
		"k",
		"EKSCluster",
		"type of cluster to create",
	)

	cmd.Flags().StringP(
		"name",
		"n",
		"example",
		"name of cluster to create",
	)

	return cmd
}

func createNewEmptyConfigFile(path string) (*os.File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	if _, err := os.Stat(absPath); err == nil {
		p := filepath.Dir(absPath)

		return nil, fmt.Errorf("%w: a furyctl.yaml configuration file already exists in %s, please remove it and try again", ErrConfigCreationFailed, p)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), iox.FullPermAccess); err != nil {
		return nil, fmt.Errorf("error creating directory: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	return out, nil
}
