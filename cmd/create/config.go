// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/cobrax"
)

var ErrConfigCreationFailed = fmt.Errorf("config creation failed")

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "scaffolds a new furyctl config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			config := cobrax.Flag[string](cmd, "config").(string)
			version := cobrax.Flag[string](cmd, "version").(string)
			kind := cobrax.Flag[string](cmd, "kind").(string)
			name := cobrax.Flag[string](cmd, "name").(string)

			data, err := configs.Tpl.ReadFile("furyctl.yaml.tpl")
			if err != nil {
				return err
			}

			tmpl, err := template.New("furyctl.yaml").Parse(string(data))
			if err != nil {
				return err
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
				return err
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
		return nil, err
	}

	if _, err := os.Stat(absPath); err == nil {
		ext := filepath.Ext(absPath)
		now := time.Now().Unix()

		trimAbsPath := absPath[:len(absPath)-len(ext)]

		absPath = fmt.Sprintf("%s.%d%s", trimAbsPath, now, ext)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, err
	}

	return os.Create(absPath)
}
