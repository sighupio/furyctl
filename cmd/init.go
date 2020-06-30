package cmd

import (
	"github.com/sirupsen/logrus"

	getter "github.com/hashicorp/go-getter"
	"github.com/spf13/cobra"
)

const (
	furyFile                    = "Furyfile.yml"
	kustomizationFile           = "kustomization.yaml"
	httpsDistributionRepoPrefix = "http::https://github.com/sighupio/fury-distribution/releases/download/"
)

var fileNames = [...]string{furyFile, kustomizationFile}
var distributionVersion string

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&distributionVersion, "version", "", "Specify the Kubernetes Fury Distribution version")
	initCmd.MarkFlagRequired("version")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the minimum distribution configuration",
	Long:  "Initialize the current directory with the minimum distribution configuration",
	Run: func(cmd *cobra.Command, args []string) {
		for _, fileName := range fileNames {
			url := httpsDistributionRepoPrefix + distributionVersion + "/" + fileName
			downloadFile(url, fileName)
		}
	},
}

func downloadFile(url string, outputFileName string) error {
	err := get(url, outputFileName, getter.ClientModeFile)
	if err != nil {
		logrus.Print(err)
	}
	return err
}
