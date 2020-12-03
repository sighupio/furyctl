package cmd

import (
	"errors"
	"fmt"

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

func validate() (err error) {
	log.Debugf("validating terraform arguments --terraform-binary %v --terraform-version %v", terraformBinaryPath, terraformVersion)
	if terraformBinaryPath != "" && terraformVersion != "" {
		log.Error("--terraform-binary and --terraform-version detected")
		return errors.New("do not use both --terraform-binary and --terraform-version")
	}

	return nil
}

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
	err = validate()
	if err != nil {
		log.Errorf("validation failed: %v", err)
		return err
	}
	err = parseConfig()
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	log.Debug("validated and configuration parsed")
	p = &project.Project{
		Path: workingDir,
	}
	bootstrapOpts := &bootstrap.Options{
		Spin:                     s,
		Project:                  p,
		ProvisionerConfiguration: config,
		TerraformOpts: &terraform.TerraformOptions{
			Version:    terraformVersion,
			BinaryPath: terraformBinaryPath,
			WorkingDir: workingDir,
			Debug:      debug,
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
	configFilePath      string
	workingDir          string
	terraformBinaryPath string
	terraformVersion    string

	bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the cluster lifecycle management",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	bootstrapInitCmd = &cobra.Command{
		Use:     "init",
		Short:   "Init a bootstrap project",
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
		Short:   "Update the bootstrap project",
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
		Short:   "Destroy the bootstrap project",
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
	bootstrapInitCmd.PersistentFlags().StringVar(&terraformBinaryPath, "terraform-binary", "", "Terraform binary to use. No compatible with --terraform-version")
	bootstrapUpdateCmd.PersistentFlags().StringVar(&terraformBinaryPath, "terraform-binary", "", "Terraform binary to use. No compatible with --terraform-version")
	bootstrapDestroyCmd.PersistentFlags().StringVar(&terraformBinaryPath, "terraform-binary", "", "Terraform binary to use. No compatible with --terraform-version")

	bootstrapInitCmd.PersistentFlags().StringVar(&terraformVersion, "terraform-version", "", "Terraform version to download and use. Incompatible if it is used along with --terrafor-binary. Example 0.12.12")
	bootstrapUpdateCmd.PersistentFlags().StringVar(&terraformVersion, "terraform-version", "", "Terraform version to download and use. Incompatible if it is used along with --terrafor-binary. Example 0.12.12")
	bootstrapDestroyCmd.PersistentFlags().StringVar(&terraformVersion, "terraform-version", "", "Terraform version to download and use. Incompatible if it is used along with --terrafor-binary. Example 0.12.12")

	bootstrapInitCmd.PersistentFlags().StringVar(&configFilePath, "config", "bootstrap.yml", "Bootstrap Configuration file path")
	bootstrapUpdateCmd.PersistentFlags().StringVar(&configFilePath, "config", "bootstrap.yml", "Bootstrap Configuration file path")
	bootstrapDestroyCmd.PersistentFlags().StringVar(&configFilePath, "config", "bootstrap.yml", "Bootstrap Configuration file path")

	bootstrapInitCmd.PersistentFlags().StringVarP(&workingDir, "workdir", "w", ".", "Working dir used to place logs and state file")
	bootstrapUpdateCmd.PersistentFlags().StringVarP(&workingDir, "workdir", "w", ".", "Working dir used to place logs and state file")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&workingDir, "workdir", "w", ".", "Working dir used to place logs and state file")

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
