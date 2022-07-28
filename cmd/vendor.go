// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
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
	Use:          "vendor",
	Short:        "Download dependencies specified in Furyfile.yml",
	Long:         "Download dependencies specified in Furyfile.yml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		list, err := config.Parse(conf.Prefix)

		if err != nil {
			logrus.Error(err)
		}

		for _, p := range list {
			logrus.Infof("using %v for package %s", p.Version, p.Name)
		}

		err = Download(list, conf.DownloadOpts)
		if err != nil {
			logrus.WithError(err).Error("ERROR DOWNLOADING")
		}

		return err
	},
}
