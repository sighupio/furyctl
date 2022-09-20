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

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sighupio/furyctl/pkg/analytics"
	"github.com/sighupio/furyctl/pkg/terraform"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func cPreDestroy(cmd *cobra.Command, args []string) (err error) {
	if cForce {
		logrus.Warn("Force destroy of the cluster project")
		return cPre(cmd, args)
	}
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

	logrus.Debug("passing pre-flight checks")
	err = parseConfig(cConfigFilePath, "Cluster")
	if err != nil {
		logrus.Errorf("error parsing configuration file: %v", err)
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	workingDirFullPath := fmt.Sprintf("%v/%v", wd, cWorkingDir)
	logrus.Debug("pre-flight checks ok!")
	prj = &project.Project{
		Path: workingDirFullPath,
	}
	clusterOpts := &cluster.Options{
		Spin:                     s,
		Project:                  prj,
		ProvisionerConfiguration: cfg,
		TerraformOpts: &terraform.Options{
			GitHubToken:        cGitHubToken,
			WorkingDir:         workingDirFullPath,
			Debug:              cDryRun,
			ReconfigureBackend: cReconfigure,
		},
	}
	clu, err = cluster.New(clusterOpts)
	if err != nil {
		logrus.Errorf("the cluster provisioner can not be initialized: %v", err)
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
	cReconfigure         bool
	cForce               bool

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
			tpl, err := configuration.Template("Cluster", strings.ToLower(cTemplateProvisioner))
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
	clusterApplyCmd = &cobra.Command{
		Use:     "apply",
		Short:   "Applies changes to the cluster project. Running for the first time creates everything. Upcoming executions only applies changes.",
		PreRunE: cPre,
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			err = prj.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute cluster init before cluster apply. %v", cWorkingDir, err)
			}

			err = clu.Update(cDryRun)
			if err != nil {
				analytics.TrackClusterApply(cGitHubToken, false, cfg.Provisioner, cDryRun)
				return err
			}
			analytics.TrackClusterApply(cGitHubToken, true, cfg.Provisioner, cDryRun)
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
	clusterApplyCmd.PersistentFlags().BoolVar(&cDryRun, "dry-run", false, "Dry run execution")

	clusterInitCmd.PersistentFlags().StringVarP(&cConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")
	clusterApplyCmd.PersistentFlags().StringVarP(&cConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")
	clusterDestroyCmd.PersistentFlags().StringVarP(&cConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")

	clusterInitCmd.PersistentFlags().StringVarP(&cWorkingDir, "workdir", "w", "./cluster", "Working directory to create and place all project files. Must not exists.")
	clusterApplyCmd.PersistentFlags().StringVarP(&cWorkingDir, "workdir", "w", "./cluster", "Working directory with all project files")
	clusterDestroyCmd.PersistentFlags().StringVarP(&cWorkingDir, "workdir", "w", "./cluster", "Working directory with all project files")

	clusterInitCmd.PersistentFlags().StringVarP(&cGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	clusterApplyCmd.PersistentFlags().StringVarP(&cGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	clusterDestroyCmd.PersistentFlags().StringVarP(&cGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")

	clusterInitCmd.PersistentFlags().BoolVar(&cReconfigure, "reconfigure", false, "Reconfigure the backend, ignoring any saved configuration")
	clusterApplyCmd.PersistentFlags().BoolVar(&cReconfigure, "reconfigure", false, "Reconfigure the backend, ignoring any saved configuration")
	clusterDestroyCmd.PersistentFlags().BoolVar(&cReconfigure, "reconfigure", false, "Reconfigure the backend, ignoring any saved configuration")

	clusterInitCmd.PersistentFlags().BoolVar(&cReset, "reset", false, "Forces the re-initialization of the project. It deletes the content of the workdir recreating everything")

	clusterDestroyCmd.PersistentFlags().BoolVar(&cForce, "force", false, "Forces the destroy of the project. Doesn't ask for confirmation")

	clusterTemplateCmd.PersistentFlags().StringVar(
		&cTemplateProvisioner,
		"provisioner",
		"",
		"Cluster provisioner, valid options are: eks, gke, vsphere",
	)

	clusterCmd.AddCommand(clusterInitCmd)
	clusterCmd.AddCommand(clusterApplyCmd)
	clusterCmd.AddCommand(clusterDestroyCmd)
	clusterCmd.AddCommand(clusterTemplateCmd)
}
