package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the client version information",
	Long:  ``,
	Run: func(_ *cobra.Command, _ []string) {
		logrus.Printf("Furyctl version %v\n", version)
		logrus.Printf("built %v from commit %v", date, commit)
	},
}
