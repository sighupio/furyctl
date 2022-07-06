// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sighupio/furyctl/internal/bootstrap"
	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sighupio/furyctl/pkg/analytics"
	"github.com/sighupio/furyctl/pkg/terraform"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func bPreDestroy(cmd *cobra.Command, args []string) (err error) {
	if bForce {
		log.Warn("Force destroy of the bootstrap project")
		return bPre(cmd, args)
	}
	fmt.Println("\r  Are you sure you want to destroy it?\n  Write 'yes' to continue")
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		os.Exit(2)
	}
	text = strings.ReplaceAll(text, "\n", "")
	if strings.Compare("yes", text) == 0 {
		return bPre(cmd, args)
	}
	return fmt.Errorf("Destroy command aborted")
}

func bPre(cmd *cobra.Command, args []string) (err error) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	handleStopSignal("bootstrap", stop)

	// viper can get the token from an environment variable: FURYCTL_TOKEN
	viper.BindPFlag("token", cmd.Flags().Lookup("token"))
	if bGitHubToken == "" { // Takes precedence the token from the cli
		bGitHubToken = viper.GetString("token")
	}

	log.Debug("passing pre-flight checks")
	err = parseConfig(bConfigFilePath, "Bootstrap")
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	workingDirFullPath := fmt.Sprintf("%v/%v", wd, bWorkingDir)
	log.Debug("pre-flight checks ok!")
	prj = &project.Project{
		Path: workingDirFullPath,
	}
	bootstrapOpts := &bootstrap.Options{
		Spin:                     s,
		Project:                  prj,
		ProvisionerConfiguration: cfg,
		TerraformOpts: &terraform.Options{
			GitHubToken:        bGitHubToken,
			WorkingDir:         workingDirFullPath,
			Debug:              debug,
			ReconfigureBackend: bReconfigure,
		},
	}
	boot, err = bootstrap.New(bootstrapOpts)
	if err != nil {
		log.Errorf("the bootstrap provisioner can not be initialized: %v", err)
		return err
	}
	return nil
}

var (
	boot *bootstrap.Bootstrap

	bConfigFilePath      string
	bWorkingDir          string
	bGitHubToken         string
	bTemplateProvisioner string
	bReset               bool
	bReconfigure         bool
	bDryRun              bool
	bForce               bool

	bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Creates the required infrastructure to deploy a battle-tested Kubernetes cluster, mostly network components",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = cmd.Help()
			if err != nil {
				return err
			}
			return nil
		},
	}
	bootstrapTemplateCmd = &cobra.Command{
		Use:   "template",
		Short: "Get a template configuration file for a specific provisioner",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if bTemplateProvisioner == "" {
				return errors.New("You must specify a provisioner")
			}
			tpl, err := configuration.Template("Bootstrap", bTemplateProvisioner)
			if err != nil {
				return err
			}
			fmt.Println(tpl)
			return nil
		},
	}
	bootstrapInitCmd = &cobra.Command{
		Use:     "init",
		Short:   "Init a the project. Creates a directory with everything in place to apply the configuration",
		PreRunE: bPre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			err = prj.Check()
			if err == nil && !bReset {
				return fmt.Errorf("the project %v seems to be already created. Choose another working directory", bWorkingDir)
			}

			err = boot.Init(bReset)
			if err != nil {
				analytics.TrackBootstrapInit(bGitHubToken, false, cfg.Provisioner)
				return err
			}
			analytics.TrackBootstrapInit(bGitHubToken, true, cfg.Provisioner)
			return nil
		},
	}
	bootstrapApplyCmd = &cobra.Command{
		Use:     "apply",
		Short:   "Applies changes to the project. Running for the first time creates everything. Upcoming executions only applies changes.",
		PreRunE: bPre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = prj.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute bootstrap init before bootstrap apply. %v", bWorkingDir, err)
			}

			err = boot.Update(bDryRun)
			if err != nil {
				analytics.TrackBootstrapApply(bGitHubToken, false, cfg.Provisioner, bDryRun)
				return err
			}
			analytics.TrackBootstrapApply(bGitHubToken, true, cfg.Provisioner, bDryRun)
			return nil
		},
	}
	bootstrapDestroyCmd = &cobra.Command{
		Use:     "destroy",
		Short:   "ATTENTION: Destroys the project. Ensure you destroy the cluster before destroying the bootstrap project.",
		PreRunE: bPreDestroy,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = prj.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute bootstrap init before cluster destroy. %v", bWorkingDir, err)
			}

			err = boot.Destroy()
			if err != nil {
				analytics.TrackBootstrapDestroy(bGitHubToken, false, cfg.Provisioner)
				return err
			}
			analytics.TrackBootstrapDestroy(bGitHubToken, true, cfg.Provisioner)
			return nil
		},
	}
)

func init() {
	bootstrapApplyCmd.PersistentFlags().BoolVar(&bDryRun, "dry-run", false, "Dry run execution")

	bootstrapInitCmd.PersistentFlags().StringVarP(&bConfigFilePath, "config", "c", "bootstrap.yml", "Bootstrap configuration file path")
	bootstrapApplyCmd.PersistentFlags().StringVarP(&bConfigFilePath, "config", "c", "bootstrap.yml", "Bootstrap configuration file path")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&bConfigFilePath, "config", "c", "bootstrap.yml", "Bootstrap configuration file path")

	bootstrapInitCmd.PersistentFlags().StringVarP(&bWorkingDir, "workdir", "w", "./bootstrap", "Working directory to create and place all project files. Must not exists.")
	bootstrapApplyCmd.PersistentFlags().StringVarP(&bWorkingDir, "workdir", "w", "./bootstrap", "Working directory with all project files")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&bWorkingDir, "workdir", "w", "./bootstrap", "Working directory with all project files")

	bootstrapInitCmd.PersistentFlags().StringVarP(&bGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	bootstrapApplyCmd.PersistentFlags().StringVarP(&bGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	bootstrapDestroyCmd.PersistentFlags().StringVarP(&bGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")

	bootstrapInitCmd.PersistentFlags().BoolVar(&bReconfigure, "reconfigure", false, "Reconfigure the backend, ignoring any saved configuration")
	bootstrapApplyCmd.PersistentFlags().BoolVar(&bReconfigure, "reconfigure", false, "Reconfigure the backend, ignoring any saved configuration")
	bootstrapDestroyCmd.PersistentFlags().BoolVar(&bReconfigure, "reconfigure", false, "Reconfigure the backend, ignoring any saved configuration")

	bootstrapInitCmd.PersistentFlags().BoolVar(&bReset, "reset", false, "Forces the re-initialization of the project. It deletes the content of the workdir recreating everything")

	bootstrapDestroyCmd.PersistentFlags().BoolVar(&bForce, "force", false, "Forces the destroy of the project. Doesn't ask for confirmation")

	bootstrapTemplateCmd.PersistentFlags().StringVar(&bTemplateProvisioner, "provisioner", "", "Bootstrap provisioner")

	bootstrapCmd.AddCommand(bootstrapInitCmd)
	bootstrapCmd.AddCommand(bootstrapApplyCmd)
	bootstrapCmd.AddCommand(bootstrapDestroyCmd)
	bootstrapCmd.AddCommand(bootstrapTemplateCmd)
	rootCmd.AddCommand(bootstrapCmd)
}
