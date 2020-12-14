package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sighupio/furyctl/pkg/terraform"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	c *cluster.Cluster
)

func clusterWarning() {
	fmt.Print(`
  Forced stop of the cluster process.
  furyctl can not guarantee the correct behavior after this
  action. Try to recover the state of the process running:

  $ furyctl cluster update

`)
}

func handleClusterSignal(c chan os.Signal) {
	go func() {
		<-c
		fmt.Println("\r  Are you sure you want to stop it?\n  Write 'yes' to force close it. Press enter to continue")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\n", "", -1)
		if strings.Compare("yes", text) == 0 {
			clusterWarning()
			os.Exit(1)
		}
		handleClusterSignal(c)
	}()
}

func preCluster(cmd *cobra.Command, args []string) (err error) {

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	handleClusterSignal(stop)

	log.Debug("passing pre-flight checks")
	err = parseConfig(clusterConfigFilePath, "Cluster")
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	wd, _ := os.Getwd()
	workingDirFullPath := fmt.Sprintf("%v/%v", wd, clusterWorkingDir)
	log.Debug("pre-flight checks ok!")
	p = &project.Project{
		Path: workingDirFullPath,
	}
	clusterOpts := &cluster.Options{
		Spin:                     s,
		Project:                  p,
		ProvisionerConfiguration: config,
		TerraformOpts: &terraform.TerraformOptions{
			GitHubToken: clusterGitHubToken,
			WorkingDir:  workingDirFullPath,
			Debug:       clusterDryRun,
		},
	}
	c, err = cluster.New(clusterOpts)
	if err != nil {
		log.Errorf("the cluster provisioner can not be initialized: %v", err)
		return err
	}
	return nil
}

var (
	clusterConfigFilePath string
	clusterDryRun         bool
	clusterWorkingDir     string
	clusterGitHubToken    string
	clusterCmd            = &cobra.Command{
		Use:   "cluster",
		Short: "Creates a battle-tested Kubernetes cluster",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	clusterInitCmd = &cobra.Command{
		Use:     "init",
		Short:   "Init the cluster project. Creates a directory with everything in place to apply the configuration",
		PreRunE: preCluster,
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			err = p.Check()
			if err == nil {
				return fmt.Errorf("the project %v seems to be already created. Choose another working directory", workingDir)
			}

			err = c.Init()
			if err != nil {
				return err
			}
			return nil
		},
	}
	clusterUpdateCmd = &cobra.Command{
		Use:     "update",
		Short:   "Applies changes to the cluster project. Running for the first time creates everything. Upcoming executions only applies changes.",
		PreRunE: preCluster,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = p.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute cluster init before cluster update. %v", workingDir, err)
			}
			err = c.Update(clusterDryRun)
			if err != nil {
				return err
			}
			return nil
		},
	}
	clusterDestroyCmd = &cobra.Command{
		Use:     "destroy",
		Short:   "ATTENTION: Destroys the cluster project",
		PreRunE: preCluster,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			err = p.Check()
			if err != nil {
				return fmt.Errorf("the project %v has to be created. Execute cluster init before cluster destroy. %v", workingDir, err)
			}
			err = c.Destroy()
			if err != nil {
				return err
			}
			return nil
		},
	}
)

func init() {
	clusterUpdateCmd.PersistentFlags().BoolVar(&clusterDryRun, "dry-run", false, "Dry run execution")

	clusterInitCmd.PersistentFlags().StringVarP(&clusterConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")
	clusterUpdateCmd.PersistentFlags().StringVarP(&clusterConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")
	clusterDestroyCmd.PersistentFlags().StringVarP(&clusterConfigFilePath, "config", "c", "cluster.yml", "Cluster configuration file path")

	clusterInitCmd.PersistentFlags().StringVarP(&clusterWorkingDir, "workdir", "w", "./cluster", "Working directory to create and place all project files. Must not exists.")
	clusterUpdateCmd.PersistentFlags().StringVarP(&clusterWorkingDir, "workdir", "w", "./cluster", "Working directory with all project files")
	clusterDestroyCmd.PersistentFlags().StringVarP(&clusterWorkingDir, "workdir", "w", "./cluster", "Working directory with all project files")

	clusterInitCmd.PersistentFlags().StringVarP(&clusterGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	clusterUpdateCmd.PersistentFlags().StringVarP(&clusterGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")
	clusterDestroyCmd.PersistentFlags().StringVarP(&clusterGitHubToken, "token", "t", "", "GitHub token to access enterprise repositories. Contact sales@sighup.io")

	clusterCmd.AddCommand(clusterInitCmd)
	clusterCmd.AddCommand(clusterUpdateCmd)
	clusterCmd.AddCommand(clusterDestroyCmd)
	rootCmd.AddCommand(clusterCmd)
}
