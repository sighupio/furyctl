// Copyright © 2018 Sighup SRL support@sighup.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/sighupio/furyctl/pkg/analytics"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	s                *spinner.Spinner
	debug            bool
	disableAnalytics bool
)

// Execute is the main entrypoint of furyctl
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Errorln(err)
		os.Exit(1)
	}
}

func init() {
	s = spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(logrus.StandardLogger().Out))
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.furyctl.yaml)")
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().Bool("debug", false, "Enables furyctl debug output")
	rootCmd.PersistentFlags().BoolVarP(&disableAnalytics, "disable", "d", false, "Disable analytics")

	cobra.OnInitialize(func() {
		analytics.Version(version)
		analytics.Disable(disableAnalytics)
	})
	viper.AutomaticEnv()
	viper.SetEnvPrefix("furyctl")
}

func bootstrapLogrus(cmd *cobra.Command) {
	d, err := cmd.Flags().GetBool("debug")

	if err != nil {
		logrus.Fatal(err)
	}

	if d {
		logrus.SetLevel(logrus.DebugLevel)
		debug = true
		return
	}
	logrus.SetLevel(logrus.InfoLevel)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "furyctl",
	Short: "A command line tool to manage cluster deployment with kubernetes",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		bootstrapLogrus(cmd)
	},
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the client version information",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Printf("Furyctl version %v\n", version)
		logrus.Printf("built %v from commit %v", date, commit)
	},
}
