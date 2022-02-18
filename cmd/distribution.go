// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	getter "github.com/hashicorp/go-getter"
	"github.com/spf13/cobra"
)

const (
	furyFile                    = "Furyfile.yml"
	kustomizationFile           = "kustomization.yaml"
	httpsDistributionRepoPrefix = "http::https://github.com/sighupio/fury-distribution/releases/download/"
)

var (
    distributionVersion string
	initKustomize bool

	distributionCmd = &cobra.Command{
		Use:   "distribution",
		Short: "Manages Kubernetes Fury Distribution, initialize Furyfile.yml and download Fury distribution modules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = cmd.Help()
			if err != nil {
				return err
			}
			return nil
		},
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize the minimum distribution configuration",
		Long:  "Initialize the current directory with the minimum distribution configuration",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// if distributionVersion is empty throw error
			if distributionVersion == "" {
				return fmt.Errorf("--version <version> flag is required")
			}

			url := httpsDistributionRepoPrefix + distributionVersion + "/" + furyFile
			err = downloadFile(url, furyFile)
			if err != nil {
				return err
			}

			if initKustomize {
				url := httpsDistributionRepoPrefix + distributionVersion + "/" + kustomizationFile
				err = downloadFile(url, kustomizationFile)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}
	downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download dependencies specified in Furyfile.yml",
		Long:  "Download dependencies specified in Furyfile.yml",
		Run: func(cmd *cobra.Command, args []string) {
			viper.SetConfigType("yml")
			viper.AddConfigPath(".")
			viper.SetConfigName(configFile)
			config := new(Furyconf)
			if err := viper.ReadInConfig(); err != nil {
				logrus.Fatalf("Error reading config file, %s", err)
			}
			err := viper.Unmarshal(config)
			if err != nil {
				logrus.Fatalf("unable to decode into struct, %v", err)
			}

			err = config.Validate()
			if err != nil {
				logrus.WithError(err).Error("ERROR VALIDATING")
			}

			list, err := config.Parse(prefix)

			if err != nil {
				//logrus.Errorln("ERROR PARSING: ", err)
				logrus.WithError(err).Error("ERROR PARSING")

			}

			err = download(list)
			if err != nil {
				//logrus.Errorln("ERROR DOWNLOADING: ", err)
				logrus.WithError(err).Error("ERROR DOWNLOADING")

			}
		},
	}
)

func downloadFile(url string, outputFileName string) error {
	err := get(url, outputFileName, getter.ClientModeFile, false)
	if err != nil {
		logrus.Print(err)
	}
	return err
}

func mergeFuryfile(url string, mergedFileName string) error {
	err := merge(url, mergedFileName, getter.ClientModeFile, false)
	if err != nil {
		logrus.Print(err)
	}
	return err
}

func init() {

	initCmd.PersistentFlags().StringVarP(&distributionVersion, "version", "v","", "Specify the Kubernetes Fury Distribution version")
	initCmd.PersistentFlags().BoolVar(&initKustomize, "kustomize", false,"Initialize kustomize.yaml file")

	downloadCmd.PersistentFlags().BoolVarP(&parallel, "parallel", "p", true, "if true enables parallel downloads")
	downloadCmd.PersistentFlags().BoolVarP(&https, "https", "H", false, "if true downloads using https instead of ssh")
	downloadCmd.PersistentFlags().StringVarP(&prefix, "prefix", "P", "", "Add filtering on download with prefix, to reduce update scope")
	
	distributionCmd.AddCommand(initCmd)
	distributionCmd.AddCommand(downloadCmd)

	rootCmd.AddCommand(distributionCmd)
}
