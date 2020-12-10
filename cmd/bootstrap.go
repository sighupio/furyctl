package cmd

import (
	"fmt"
	"os"

	"github.com/sighupio/furyctl/internal/bootstrap"
	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sighupio/furyctl/pkg/terraform"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	p      *project.Project
	config *configuration.Configuration
	b      *bootstrap.Bootstrap
)

func parseConfig() (err error) {
	log.Debugf("parsing configuration file %v", configFilePath)
	config, err = configuration.Parse(configFilePath)
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	return nil
}

func pre(cmd *cobra.Command, args []string) (err error) {
	log.Debug("passing pre-flight checks")
	err = parseConfig()
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	wd, _ := os.Getwd()
	workingDirFullPath := fmt.Sprintf("%v/%v", wd, workingDir)
	log.Debug("pre-flight checks ok!")
	p = &project.Project{
		Path: workingDirFullPath,
	}
	bootstrapOpts := &bootstrap.Options{
		Spin:                     s,
		Project:                  p,
		ProvisionerConfiguration: config,
		TerraformOpts: &terraform.TerraformOptions{
			GitHubToken: gitHubToken,
			WorkingDir:  workingDirFullPath,
			Debug:       debug,
		},
	}
	b, err = bootstrap.New(bootstrapOpts)
	if err != nil {
		log.Errorf("the bootstrap provisioner can not be initialized: %v", err)
		return err
	}
	return nil
}

var (
	configFilePath string
	workingDir     string
	gitHubToken    string

	bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Creates the required infrastructure to deploy a battle-tested Kubernetes cluster, mostly network components",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	bootstrapInitCmd = &cobra.Command{
		Use:     "init",
		Short:   "Init a the project. Creates a directory with everything in place to apply the configuration",
		PreRunE: pre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			err = p.Check()
			if err == nil {
				return fmt.Errorf("the project %v seems to be already created. Choose another working directory", workingDir)
			}

			err = b.Init()
			if err != nil {
				return err
			}
			return nil
		},
	}
	bootstrapUpdateCmd = &cobra.Command{
		Use:     "update",
		Short:   "Applies changes to the project. Running for the first time creates everything. Upcoming executions only applies changes.",
		PreRunE: pre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = p.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute bootstrap init before bootstrap update. %v", workingDir, err)
			}
			err = b.Update()
			if err != nil {
				return err
			}
			return nil
		},
	}
	bootstrapDestroyCmd = &cobra.Command{
		Use:     "destroy",
		Short:   "ATTENTION: Destroys the project. Ensure you destroy the cluster before destroying the bootstrap project.",
		PreRunE: pre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = p.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute bootstrap init before cluster destroy. %v", workingDir, err)
			}
			err = b.Destroy()
			if err != nil {
				return err
			}
			return nil
		},
	}
)

func init() {
	bootstrapInitCmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", "bootstrap.yml", "Bootstrap configuration file path")
	bootstrapUpdateCmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", "bootstrap.yml", "Bootstrap configuration file path")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", "bootstrap.yml", "Bootstrap configuration file path")

	bootstrapInitCmd.PersistentFlags().StringVarP(&workingDir, "workdir", "w", "", "Working directory to create and place all project files. Must not exists.")
	bootstrapUpdateCmd.PersistentFlags().StringVarP(&workingDir, "workdir", "w", "", "Working directory with all project files")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&workingDir, "workdir", "w", "", "Working directory with all project files")

	bootstrapInitCmd.PersistentFlags().StringVarP(&gitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	bootstrapUpdateCmd.PersistentFlags().StringVarP(&gitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&gitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")

	bootstrapInitCmd.MarkPersistentFlagRequired("config")
	bootstrapUpdateCmd.MarkPersistentFlagRequired("config")
	bootstrapDestroyCmd.MarkPersistentFlagRequired("config")

	bootstrapInitCmd.MarkPersistentFlagRequired("workdir")
	bootstrapUpdateCmd.MarkPersistentFlagRequired("workdir")
	bootstrapDestroyCmd.MarkPersistentFlagRequired("workdir")

	bootstrapCmd.AddCommand(bootstrapInitCmd)
	bootstrapCmd.AddCommand(bootstrapUpdateCmd)
	bootstrapCmd.AddCommand(bootstrapDestroyCmd)
	rootCmd.AddCommand(bootstrapCmd)
}
