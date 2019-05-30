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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sighup-io/furyctl/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	rootCmd.AddCommand(scaffoldCommand)
}

var scaffoldCommand = &cobra.Command{
	Use:   "scaffold",
	Short: "create required folders and files",
	Long:  "scaffold a fury project creating required folders and files",
	Run: func(cmd *cobra.Command, args []string) {
		scaffoldTerraformModules()
		scaffoldK8sManifests()
		scaffoldAnsibleRules()
		createGitIgnore()
	},
}

func scaffoldK8sManifests() {
	util.CreateFolderInCurrentDirectory("manifests")

	type kustomizationFile struct {
		Bases []string `yaml:"bases"`
	}

	currentDir, _ := os.Getwd()
	absoluteVendorPath := filepath.Join(currentDir, "vendor")
	if _, err := os.Stat(absoluteVendorPath); os.IsNotExist(err) {
		fmt.Println("no 'vendor' folder found, run `furyctl install'")
		os.Exit(0)
	}

	bases, _ := util.FindBasesFromVendor(absoluteVendorPath)

	var relativeBases []string
	const vendorPathRelativeToManifestsFolder = "../vendor"
	for _, b := range bases {
		s := strings.Replace(b, absoluteVendorPath, vendorPathRelativeToManifestsFolder, 1)
		relativeBases = append(relativeBases, s)
	}

	content, err := yaml.Marshal(&kustomizationFile{Bases: relativeBases})
	if err != nil {
		panic(err)
	}

	util.SafeWriteFileOrExit(filepath.Join(currentDir, "manifests/kustomization.yml"), content)
}

func scaffoldAnsibleRules() {
	util.CreateFolderInCurrentDirectory("ansible")
}

func scaffoldTerraformModules() {
	util.CreateFolderInCurrentDirectory("terraform")
}

func scaffoldSecrect() {
	util.CreateFolderInCurrentDirectory("secrets")
}

func createGitIgnore() {
	content := []byte(`*.retry
.terraform
*.tfstate
*.backup
`)
	util.SafeWriteFileOrExit(".gitignore", content)
}
