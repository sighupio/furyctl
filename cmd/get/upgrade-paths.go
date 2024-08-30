package get

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	distroconf "github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

func NewUpgradePathsCmd() *cobra.Command {
	var cmdEvent analytics.Event

	upgradePathsCmd := &cobra.Command{
		Use:   "upgrade-paths",
		Short: "Get available upgrade paths for the kind and version defined in the configuration file or a custom one.",
		Long: `Get available upgrade paths for the kind and version defined in the configuration file or a custom one. If a from version or kind are specified the command will give the upgrade path for those instaed.
 Examples:
 - furyctl get upgrade-paths                               	will show the available upgrade paths for the kind a distribution version defined in the configuration file (furyctl.yaml by default)
 - furyctl get upgrade-paths --from vX.Y.Z                 	will show the available upgrade paths for the kind defined in the configuration file but for the version X.Y.Z instead.
 - furyctl get upgrade-paths --kind OnPremises             	will show the available upgrade paths for the version defined in the configuration file but for the OnPremises kind, even if the cluster is an EKSCluster, for example.
 - furyctl get upgrade-paths --kind OnPremises --from X.Y.X	will show the available upgrade paths for the version X.Y.Z of the OnPremises kind, without reading the configuration file.
 `,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			// Get flags.
			debug := viper.GetBool("debug")
			binPath := viper.GetString("bin-path")
			furyctlPath := viper.GetString("config")
			outDir := viper.GetString("outdir")
			distroLocation := viper.GetString("distro-location")
			gitProtocol := viper.GetString("git-protocol")
			skipDepsDownload := viper.GetBool("skip-deps-download")
			skipDepsValidation := viper.GetBool("skip-deps-validation")
			fromVersion := viper.GetString("from")
			kind := viper.GetString("kind")
			// Get Current dir.
			logrus.Debug("Getting current directory path...")

			currentDir, err := os.Getwd()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting current directory: %w", err)
			}

			// Get home dir.
			logrus.Debug("Getting Home directory path...")
			homeDir, err := os.UserHomeDir()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting user home directory: %w", err)
			}

			if binPath == "" {
				binPath = path.Join(homeDir, ".furyctl", "bin")
			}

			parsedGitProtocol := (git.Protocol)(gitProtocol)

			if outDir == "" {
				outDir = currentDir
			}

			// Init packages.
			execx.Debug = debug

			// Check that the version passed by the user is semVer valid.
			if fromVersion != "" {
				if _, err := semver.NewVersion(fromVersion); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("'%s' is not a valid version string: %w", fromVersion, err)
				}
			}

			// Load the configuration file only if we need to get some of the values from there.
			// It's time consuming and requires downloading stuff from Internet.
			if kind == "" || fromVersion == "" {
				executor := execx.NewStdExecutor()

				distrodl := &dist.Downloader{}
				depsvl := dependencies.NewValidator(executor, binPath, furyctlPath, false)

				// Init first half of collaborators.
				client := netx.NewGoGetterClient()

				if distroLocation == "" {
					distrodl = dist.NewCachingDownloader(client, outDir, parsedGitProtocol, "")
				} else {
					distrodl = dist.NewDownloader(client, parsedGitProtocol, "")
				}

				// Validate base requirements.
				if err := depsvl.ValidateBaseReqs(); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating requirements: %w", err)
				}

				// Download the distribution.
				logrus.Info("Downloading distribution...")

				res, err := distrodl.Download(distroLocation, furyctlPath)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while downloading distribution: %w", err)
				}

				basePath := path.Join(outDir, ".furyctl", res.MinimalConf.Metadata.Name)

				// Init second half of collaborators.
				depsdl := dependencies.NewCachingDownloader(client, homeDir, basePath, binPath, parsedGitProtocol)

				// Validate the furyctl.yaml file.
				logrus.Info("Validating configuration file...")
				if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating configuration file: %w", err)
				}

				// Download the dependencies.
				if !skipDepsDownload {
					logrus.Info("Downloading dependencies...")
					if _, err := depsdl.DownloadTools(res.DistroManifest); err != nil {
						cmdEvent.AddErrorMessage(ErrDownloadDependenciesFailed)
						tracker.Track(cmdEvent)

						return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, err)
					}
				}

				// Validate the dependencies, unless explicitly told to skip it.
				if !skipDepsValidation {
					logrus.Info("Validating dependencies...")
					if err := depsvl.Validate(res); err != nil {
						cmdEvent.AddErrorMessage(err)
						tracker.Track(cmdEvent)

						return fmt.Errorf("error while validating dependencies: %w", err)
					}
				}

				logrus.Debugf("either kind or fromVersion is not specified, reading them from the configuration file in path %s.", furyctlPath)
				furyctlConf, err := yamlx.FromFileV3[distroconf.Furyctl](furyctlPath)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while reading configuration file in path %s: %w", furyctlPath, err)
				}
				if kind == "" {
					kind = furyctlConf.Kind
					logrus.Debugf("got kind %s from the configuration file.", kind)
				}
				if fromVersion == "" {
					fromVersion = furyctlConf.Spec.DistributionVersion
					logrus.Debugf("got version %s from the configuration file.", fromVersion)
				}
			}

			// We don't need the starting v in the version. Drop it if the user passes it.
			fromVersion, _ = strings.CutPrefix(fromVersion, "v")

			globPattern := fmt.Sprintf("%s/%s/%s-*", "upgrades", strings.ToLower(kind), fromVersion)
			availablePaths, err := fs.Glob(configs.Tpl, globPattern)
			logrus.Debug("found folders: ", availablePaths)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting the upgrade paths for version %s: %w", fromVersion, err)
			}

			if len(availablePaths) > 0 {
				var targetVersions []string
				for _, match := range availablePaths {
					f, err := fs.Stat(configs.Tpl, match)
					if err != nil {
						cmdEvent.AddErrorMessage(err)
						tracker.Track(cmdEvent)

						return fmt.Errorf("error while checking filesystem: %w", err)
					}
					if f.IsDir() {
						fromToVersions := strings.Split(filepath.Base(match), "-")
						toVersion := fromToVersions[len(fromToVersions)-1]
						targetVersions = append(targetVersions, toVersion)
					}
				}
				logrus.Infof("Available upgrade paths for version %s of kind %s are: %s", fromVersion, kind, strings.Join(targetVersions, ", "))
			} else {
				logrus.Infof("There are no upgrade paths available for version %s of kind %s", fromVersion, kind)
			}
			cmdEvent.AddSuccessMessage("upgrade paths successfully retrieved")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	upgradePathsCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	upgradePathsCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	upgradePathsCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	upgradePathsCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the binaries",
	)

	upgradePathsCmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	upgradePathsCmd.Flags().String(
		"from",
		"",
		"Show upgrade paths for the version specified (eg. 1.29.2) instead of the distribution version in the configuration file.",
	)

	upgradePathsCmd.Flags().StringP(
		"kind",
		"k",
		"",
		"Show upgrade paths for the kind of cluster specified (eg: EKSCluster, KFDDistribution, OnPremises) instead of the kind defined in the configuration file.",
	)

	return upgradePathsCmd
}
