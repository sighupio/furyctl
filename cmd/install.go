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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	getter "github.com/hashicorp/go-getter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	configFile              = "Furyfile"
	protocol                = "git::ssh://"
	modulesRepo             = "git@git.sighup.io/sighup/fury-modules.git"
	rolesRepo               = "git@git.sighup.io/sighup/fury-roles.git"
	katalogRepo             = "git@git.sighup.io/sighup/fury-katalog.git"
	modulesRepoDev          = "git@git.incubator.sh/sighup/fury-modules.git"
	rolesRepoDev            = "git@git.incubator.sh/sighup/fury-roles.git"
	katalogRepoDev          = "git@git.incubator.sh/sighup/fury-katalog.git"
	defaultVendorFolderName = "vendor"
)

// Furyconf is reponsible for the structure of the Furyfile
type Furyconf struct {
	VendorFolderName string    `yaml:"vendorFolderName"`
	SSHKeyPath       string    `yaml:"sshKeyPath"`
	Roles            []Package `yaml:"roles"`
	Modules          []Package `yaml:"modules"`
	Bases            []Package `yaml:"bases"`
}

// Package is the type to contain the definition of a single package
type Package struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	url     string
	dit     string
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.PersistentFlags().Bool("dev", false, "Download from development repo (git.incubator.sh)")
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Download dependencies specified in Furyfile.yml",
	Long:  "Download dependencies specified in Furyfile.yml",
	Run: func(cmd *cobra.Command, args []string) {
		dev := cmd.Flag("dev").Changed
		viper.SetConfigType("yml")
		viper.AddConfigPath(".")
		viper.SetConfigName(configFile)
		configuration := new(Furyconf)
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file, %s", err)
		}
		err := viper.Unmarshal(configuration)
		if err != nil {
			log.Fatalf("unable to decode into struct, %v", err)
		}

		err = configuration.Validate()
		if err != nil {
			log.Println("ERROR VALIDATING: ", err)
		}

		err = configuration.Download(dev)
		if err != nil {
			log.Println("ERROR DOWNLOADING: ", err)
		}
	},
}

// Download is the main function to put all the files in vendor folder
func (f *Furyconf) Download(dev bool) error {
	var rolesRepoURL string
	var modulesRepoURL string
	var katalogRepoURL string

	if dev {
		rolesRepoURL = rolesRepoDev
		modulesRepoURL = modulesRepoDev
		katalogRepoURL = katalogRepoDev
	} else {
		rolesRepoURL = rolesRepo
		modulesRepoURL = modulesRepo
		katalogRepoURL = katalogRepo
	}

	for _, v := range f.Roles {
		url := fmt.Sprintf("%s%s//%s?ref=%s", protocol, rolesRepoURL, v.Name, v.Version)
		dir := fmt.Sprintf("%s/%s/%s", f.VendorFolderName, "roles", v.Name)
		err := get(url, dir)
		if err != nil {
			return err
		}
	}
	for _, v := range f.Modules {
		url := fmt.Sprintf("%s%s//%s?ref=%s", protocol, modulesRepoURL, v.Name, v.Version)
		dir := fmt.Sprintf("%s/%s/%s", f.VendorFolderName, "modules", v.Name)
		err := get(url, dir)
		if err != nil {
			return err
		}
	}
	for _, v := range f.Bases {
		url := fmt.Sprintf("%s%s//%s?ref=%s", protocol, katalogRepoURL, v.Name, v.Version)
		dir := fmt.Sprintf("%s/%s/%s", f.VendorFolderName, "bases", v.Name)
		err := get(url, dir)
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate is used for validation of configuration and initization of default paramethers
func (f *Furyconf) Validate() error {
	if f.VendorFolderName == "" {
		f.VendorFolderName = defaultVendorFolderName
	}
	return nil
}

func get(src, dest string) error {
	fmt.Println("DOWNLOADING...\nSRC: ", src, "\nDST: ", dest)
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	client := &getter.Client{
		Src:  src,
		Dst:  dest,
		Pwd:  pwd,
		Mode: getter.ClientModeDir,
	}
	return client.Get()
}

func addSSHKey(url, sshKeyPath string) string {
	if sshKeyPath != "" {
		sshKeyData, err := ioutil.ReadFile(sshKeyPath)
		if err != nil {
			log.Println("couldn't find or read provided SSHKEY, ignoring error and continuing")
			log.Println("ERR:", err)
			return url
		}
		url = fmt.Sprintf("%s&sshkey=%s", url, base64.StdEncoding.EncodeToString(sshKeyData))
	}
	return url
}
