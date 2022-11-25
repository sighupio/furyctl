// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	vendorCmd.PersistentFlags().BoolVarP(&conf.DownloadOpts.Https, "https", "H", false, "download using HTTPS instead of SSH protocol. Use when SSH traffic is being blocked or when SSH client has not been configured\nset the GITHUB_TOKEN environment variable with your token to use authentication while downloading")
	vendorCmd.PersistentFlags().StringVarP(&conf.Prefix, "prefix", "P", "", "download modules that start with prefix only to reduce download scope. Example:\nfuryctl vendor -P mon\nwill download all modules that start with 'mon', like 'monitoring', and ignore the rest")
	rootCmd.AddCommand(vendorCmd)
}

var conf = Config{}

type Config struct {
	Packages     []Package
	DownloadOpts DownloadOpts
	Prefix       string
}

// vendorCmd represents the vendor command
var vendorCmd = &cobra.Command{
	Use:           "vendor",
	Short:         "download KFD modules and dependencies specified in Furyfile.yml",
	Long:          "download KFD modules and dependencies specified in Furyfile.yml",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		viper.SetConfigType("yml")
		viper.AddConfigPath(".")
		viper.SetConfigName(configFile)
		config := new(Furyconf)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}

		if err := viper.Unmarshal(config); err != nil {
			return err
		}

		if err := config.Validate(); err != nil {
			return err
		}

		list, err := config.Parse(conf.Prefix)

		if err != nil {
			return err
		}

		for _, p := range list {
			if p.Version == "" {
				logrus.Warnf("package '%s' has no version specified. Will download the default git branch", p.Name)
			} else {
				logrus.Infof("using version '%v' for package '%s'", p.Version, p.Name)
			}
		}

		return Download(list, conf.DownloadOpts)
	},
}
