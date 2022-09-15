// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/yaml"
)

var errHasValidationErrors = fmt.Errorf("furyctl.yaml contains validation errors")

func NewConfigCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Validate furyctl.yaml file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug := false

			if cmd.Flag("debug") != nil {
				debug = cmd.Flag("debug").Value.String() == "true"
			}

			furyctlPath := cmd.Flag("config").Value.String()
			distroLocation := cmd.Flag("distro-location").Value.String()

			minimalConf, err := yaml.FromFileV3[distribution.FuryctlConfig](furyctlPath)
			if err != nil {
				return err
			}

			furyctlConfVersion := minimalConf.Spec.DistributionVersion

			if version != "dev" {
				furyctlBinVersion, err := semver.NewVersion(version)
				if err != nil {
					return err
				}

				sameMinors := semver.SameMinor(furyctlConfVersion, furyctlBinVersion)

				if !sameMinors {
					logrus.Warnf(
						"this version of furyctl ('%s') does not support distribution version '%s', results may be inaccurate",
						furyctlBinVersion,
						furyctlConfVersion,
					)
				}
			}

			if distroLocation == "" {
				distroLocation = fmt.Sprintf(DefaultBaseUrl, furyctlConfVersion.String())
			}

			repoPath, err := downloadDirectory(distroLocation)
			if err != nil {
				return err
			}
			if !debug {
				defer cleanupTempDir(filepath.Base(repoPath))
			}

			kfdPath := filepath.Join(repoPath, "kfd.yaml")
			kfdManifest, err := yaml.FromFileV3[distribution.Manifest](kfdPath)
			if err != nil {
				return err
			}

			if !semver.SamePatch(furyctlConfVersion, kfdManifest.Version) {
				return fmt.Errorf(
					"minor versions mismatch: furyctl.yaml has %s, but furyctl has %s",
					furyctlConfVersion.String(),
					kfdManifest.Version.String(),
				)
			}

			schemaPath, err := getSchemaPath(repoPath, minimalConf)
			if err != nil {
				return err
			}

			defaultPath := getDefaultPath(repoPath)

			defaultedFuryctlPath, err := mergeConfigAndDefaults(furyctlPath, defaultPath)
			if err != nil {
				return err
			}
			if !debug {
				defer cleanupTempDir(filepath.Base(defaultedFuryctlPath))
			}

			schema, err := santhosh.LoadSchema(schemaPath)
			if err != nil {
				return err
			}

			hasErrors := error(nil)
			conf, err := yaml.FromFileV3[any](defaultedFuryctlPath)
			if err != nil {
				return err
			}

			if err := schema.ValidateInterface(conf); err != nil {
				logrus.Debugf("Config file: %s", defaultedFuryctlPath)

				fmt.Println(err)

				hasErrors = errHasValidationErrors
			}

			if hasErrors == nil {
				fmt.Println("Validation succeeded")
			}

			return hasErrors
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	return cmd
}

func mergeConfigAndDefaults(furyctlFilePath string, defaultsFilePath string) (string, error) {
	defaultsFile, err := yaml.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrYamlUnmarshalFile, err)
	}

	furyctlFile, err := yaml.FromFileV2[map[any]any](furyctlFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrYamlUnmarshalFile, err)
	}

	defaultsModel := merge.NewDefaultModel(defaultsFile, ".data")
	distributionModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")

	distroMerger := merge.NewMerger(defaultsModel, distributionModel)

	defaultedDistribution, err := distroMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMergeDistroConfig, err)
	}

	furyctlModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")
	defaultedDistributionModel := merge.NewDefaultModel(defaultedDistribution, ".data")

	furyctlMerger := merge.NewMerger(furyctlModel, defaultedDistributionModel)

	defaultedFuryctl, err := furyctlMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMergeCompleteConfig, err)
	}

	outYaml, err := yaml.MarshalV2(defaultedFuryctl)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrYamlMarshalFile, err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-defaulted-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")
	if err := os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return "", fmt.Errorf("%w: %v", ErrWriteFile, err)
	}

	return confPath, nil
}
