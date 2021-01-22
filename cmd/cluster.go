// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
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

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sighupio/furyctl/pkg/analytics"
	"github.com/sighupio/furyctl/pkg/terraform"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func cPreDestroy(cmd *cobra.Command, args []string) (err error) {
	fmt.Println("\r  Are you sure you want to destroy the cluster?\n  Write 'yes' to continue")
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		os.Exit(2)
	}
	text = strings.ReplaceAll(text, "\n", "")
	if strings.Compare("yes", text) == 0 {
		return cPre(cmd, args)
	}
	return fmt.Errorf("Destroy command aborted")
}

func cPre(cmd *cobra.Command, args []string) (err error) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	handleStopSignal("cluster", stop)

	// viper can get the token from an environment variable: FURYCTL_TOKEN
	viper.BindPFlag("token", cmd.Flags().Lookup("token"))
	if cGitHubToken == "" { // Takes precedence the token from the cli
		cGitHubToken = viper.GetString("token")
	}

	log.Debug("passing pre-flight checks")
	err = parseConfig(cConfigFilePath, "Cluster")
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	workingDirFullPath := fmt.Sprintf("%v/%v", wd, cWorkingDir)
	log.Debug("pre-flight checks ok!")
	prj = &project.Project{
		Path: workingDirFullPath,
	}
	clusterOpts := &cluster.Options{
		Spin:                     s,
		Project:                  prj,
		ProvisionerConfiguration: cfg,
		TerraformOpts: &terraform.Options{
			GitHubToken: cGitHubToken,
			WorkingDir:  workingDirFullPath,
			Debug:       cDryRun,
		},
	}
	clu, err = cluster.New(clusterOpts)
	if err != nil {
		log.Errorf("the cluster provisioner can not be initialized: %v", err)
		return err
	}
	return nil
}

var (
	clu *cluster.Cluster

	cConfigFilePath      string
	cWorkingDir          string
	cGitHubToken         string
	cTemplateProvisioner string
	cDryRun              bool
	cReset               bool

	clusterCmd = &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = cmd.Help()
			if err != nil {
				return err
			}
			return nil
		},
	}
	clusterTemplateCmd = &cobra.Command{
		Use:   "template",
		Short: "Get a template configuration file for a specific provisioner",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if cTemplateProvisioner == "" {
				return errors.New("You must specify a provisioner")
			}
			tpl, err := configuration.Template("Cluster", cTemplateProvisioner)
			if err != nil {
				return err
			}
			fmt.Println(tpl)
			return nil
		},
	}
	clusterInitCmd = &cobra.Command{
		Use:     "init",
		Short:   "Init the cluster project. Creates a directory with everything in place to apply the configuration",
		PreRunE: cPre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			err = prj.Check()
			if err == nil && !cReset {
				return fmt.Errorf("the project %v seems to be already created. Choose another working directory", cWorkingDir)
			}

			err = clu.Init(cReset)
			if err != nil {
				analytics.TrackClusterInit(cGitHubToken, false, cfg.Provisioner)
				return err
			}
			analytics.TrackClusterInit(cGitHubToken, true, cfg.Provisioner)
			return nil
		},
	}
	clusterUpdateCmd = &cobra.Command{
		Use:     "update",
		Short:   "Applies changes to the cluster project. Running for the first time creates everything. Upcoming executions only applies changes.",
		PreRunE: cPre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			err = prj.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute cluster init before cluster update. %v", cWorkingDir, err)
			}

			err = clu.Update(cDryRun)
			if err != nil {
				analytics.TrackClusterUpdate(cGitHubToken, false, cfg.Provisioner, cDryRun)
				return err
			}
			analytics.TrackClusterUpdate(cGitHubToken, true, cfg.Provisioner, cDryRun)
			return nil
		},
	}
	clusterDestroyCmd = &cobra.Command{
		Use:     "destroy",
		Short:   "ATTENTION: Destroys the cluster project",
		PreRunE: cPreDestroy,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = prj.Check()

			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute cluster init before cluster destroy. %v", cWorkingDir, err)
			}

			err = clu.Destroy()
			if err != nil {
				analytics.TrackClusterDestroy(cGitHubToken, false, cfg.Provisioner)
				return err
			}
			analytics.TrackClusterDestroy(cGitHubToken, true, cfg.Provisioner)
			return nil
		},
	}
)

func init() {
	clusterUpdateCmd.PersistentFlags().BoolVar(&cDryRun, "dry-run", false, "Dry run execution")

	clusterInitCmd.PersistentFlags().StringVarP(&cConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")
	clusterUpdateCmd.PersistentFlags().StringVarP(&cConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")
	clusterDestroyCmd.PersistentFlags().StringVarP(&cConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")

	clusterInitCmd.PersistentFlags().StringVarP(&cWorkingDir, "workdir", "w", "./cluster", "Working directory to create and place all project files. Must not exists.")
	clusterUpdateCmd.PersistentFlags().StringVarP(&cWorkingDir, "workdir", "w", "./cluster", "Working directory with all project files")
	clusterDestroyCmd.PersistentFlags().StringVarP(&cWorkingDir, "workdir", "w", "./cluster", "Working directory with all project files")

	clusterInitCmd.PersistentFlags().StringVarP(&cGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	clusterUpdateCmd.PersistentFlags().StringVarP(&cGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	clusterDestroyCmd.PersistentFlags().StringVarP(&cGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")

	clusterInitCmd.PersistentFlags().BoolVar(&cReset, "reset", false, "Forces the re-initialization of the project. It deletes the content of the workdir recreating everything")

	clusterTemplateCmd.PersistentFlags().StringVar(&cTemplateProvisioner, "provisioner", "", "Cluster provisioner")

	clusterCmd.AddCommand(clusterInitCmd)
	clusterCmd.AddCommand(clusterUpdateCmd)
	clusterCmd.AddCommand(clusterDestroyCmd)
	clusterCmd.AddCommand(clusterTemplateCmd)
	rootCmd.AddCommand(clusterCmd)
}
