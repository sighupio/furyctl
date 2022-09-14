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
	vendorCmd.PersistentFlags().BoolVarP(&conf.DownloadOpts.Https, "https", "H", false, "if true downloads using https instead of ssh")
	vendorCmd.PersistentFlags().StringVarP(&conf.Prefix, "prefix", "P", "", "Add filtering on download with prefix, to reduce update scope")
	vendorCmd.PersistentFlags().BoolVarP(&conf.DownloadOpts.Parallel, "parallel", "p", true, "if true enables parallel downloads")
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
	Short:         "Download dependencies specified in Furyfile.yml",
	Long:          "Download dependencies specified in Furyfile.yml",
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
				logrus.Warnf("package %s has no version specified. Using default branch from remote.", p.Name)
			} else {
				logrus.Infof("using %v for package %s", p.Version, p.Name)
			}
		}

		return Download(list, conf.DownloadOpts)
	},
}
