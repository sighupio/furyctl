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
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(vendorCmd)
	vendorCmd.PersistentFlags().BoolVarP(&parallel, "parallel", "p", true, "if true enables parallel downloads")
	vendorCmd.PersistentFlags().BoolVarP(&https, "https", "H", false, "if true downloads using https instead of ssh")
}

// vendorCmd represents the vendor command
var vendorCmd = &cobra.Command{
	Use:   "vendor",
	Short: "Download dependencies specified in Furyfile.yml",
	Long:  "Download dependencies specified in Furyfile.yml",
	Run: func(cmd *cobra.Command, args []string) {
		viper.SetConfigType("yml")
		viper.AddConfigPath(".")
		viper.SetConfigName(configFile)
		config := new(Furyconf)
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file, %s", err)
		}
		err := viper.Unmarshal(config)
		if err != nil {
			log.Fatalf("unable to decode into struct, %v", err)
		}

		err = config.Validate()
		if err != nil {
			log.Println("ERROR VALIDATING: ", err)
		}

		list, err := config.Parse()
		if err != nil {
			log.Println("ERROR PARSING: ", err)
		}
		err = download(list)
		if err != nil {
			log.Println("ERROR DOWNLOADING: ", err)
		}
	},
}
