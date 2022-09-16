package validate

import (
	"errors"
	"fmt"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrSystemDepsValidation      = errors.New("error while validating system dependencies")
	ErrEnvironmentDepsValidation = errors.New("error while validating environment dependencies")
)

var execCommand = exec.Command

func NewDependenciesCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Validate furyctl.yaml file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug := false

			if cmd.Flag("debug") != nil {
				debug = cmd.Flag("debug").Value.String() == "true"
			}

			furyctlPath := cmd.Flag("config").Value.String()
			distroLocation := cmd.Flag("distro-location").Value.String()

			minimalConf, err := yaml.FromFileV3[distribution.FuryctlConfig](furyctlPath)
			if err != nil {
				return err
			}

			furyctlConfVersion := minimalConf.Spec.DistributionVersion

			if version != "dev" {
				furyctlBinVersion, err := semver.NewVersion(version)
				if err != nil {
					return err
				}

				sameMinors := semver.SameMinor(furyctlConfVersion, furyctlBinVersion)

				if !sameMinors {
					logrus.Warnf(
						"this version of furyctl ('%s') does not support distribution version '%s', results may be inaccurate",
						furyctlBinVersion,
						furyctlConfVersion,
					)
				}
			}

			if distroLocation == "" {
				distroLocation = fmt.Sprintf(DefaultBaseUrl, furyctlConfVersion.String())
			}

			repoPath, err := downloadDirectory(distroLocation)
			if err != nil {
				return err
			}
			if !debug {
				defer cleanupTempDir(filepath.Base(repoPath))
			}

			kfdPath := filepath.Join(repoPath, "kfd.yaml")
			kfdManifest, err := yaml.FromFileV3[distribution.Manifest](kfdPath)
			if err != nil {
				return err
			}

			logrus.Debugln("Checking system dependencies")
			if err = validateSystemDependencies(kfdManifest); err != nil {
				return err
			}

			logrus.Debugln("Checking environment dependencies")
			if err = validateEnvDependencies(minimalConf.Kind); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	return cmd
}

func validateEnvDependencies(kind string) error {
	errs := make([]error, 0)

	if kind == "EKSCluster" {
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
			missingAccessKeyIdErr := fmt.Errorf("missing environment variable with key: AWS_ACCESS_KEY_ID")
			logrus.Error(missingAccessKeyIdErr)
			errs = append(errs, missingAccessKeyIdErr)
		}

		if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
			missingSecretAccessKeyErr := fmt.Errorf("missing environment variable with key: AWS_SECRET_ACCESS_KEY")
			logrus.Error(missingSecretAccessKeyErr)
			errs = append(errs, missingSecretAccessKeyErr)
		}

		if os.Getenv("AWS_DEFAULT_REGION") == "" {
			missingDefaultRegionErr := fmt.Errorf("missing environment variable with key: AWS_DEFAULT_REGION")
			logrus.Error(missingDefaultRegionErr)
			errs = append(errs, missingDefaultRegionErr)
		}
	}

	if len(errs) > 0 {
		return ErrEnvironmentDepsValidation
	}

	return nil
}

func validateSystemDependencies(kfdManifest distribution.Manifest) error {
	errs := make([]error, 0)

	err := checkAnsibleVersion(kfdManifest.Tools.Ansible)
	if err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	err = checkTerraformVersion(kfdManifest.Tools.Terraform)
	if err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	err = checkKubectlVersion(kfdManifest.Tools.Kubectl)
	if err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	err = checkKustomizeVersion(kfdManifest.Tools.Kustomize)
	if err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	err = checkFuryagentVersion(kfdManifest.Tools.Furyagent)
	if err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return ErrSystemDepsValidation
	}

	return nil
}

func checkAnsibleVersion(ver string) error {
	out, err := execCommand("ansible", "--version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("ansible \\[.*]")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get ansible version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get ansible version from system")
	}

	systemAnsibleVersion := strings.TrimRight(versionStringTokens[len(versionStringTokens)-1], "]")

	if systemAnsibleVersion != ver {
		return fmt.Errorf("ansible version on system: %s, required version: %s", systemAnsibleVersion, ver)
	}

	return nil
}

func checkTerraformVersion(ver string) error {
	out, err := execCommand("terraform", "--version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("Terraform .*")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get terraform version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get terraform version from system")
	}

	systemTerraformVersion := strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "v")

	if systemTerraformVersion != ver {
		return fmt.Errorf("terraform version on system: %s, required version: %s", systemTerraformVersion, ver)
	}

	return nil
}

func checkKubectlVersion(ver string) error {
	out, err := execCommand("kubectl", "version", "--client").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("GitVersion:\"([^\"]*)\"")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get kubectl version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, ":")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get kubectl version from system")
	}

	systemKubectlVersion := strings.TrimRight(
		strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "\"v"),
		"\"",
	)

	if systemKubectlVersion != ver {
		return fmt.Errorf("kubectl version on system: %s, required version: %s", systemKubectlVersion, ver)
	}

	return nil
}

func checkKustomizeVersion(ver string) error {
	out, err := execCommand("kustomize", "version", "--short").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("kustomize/v(\\S*)")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get kustomize version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, "/")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get kustomize version from system")
	}

	systemKustomizeVersion := strings.TrimLeft(versionStringTokens[len(versionStringTokens)-1], "v")

	if systemKustomizeVersion != ver {
		return fmt.Errorf("kustomize version on system: %s, required version: %s", systemKustomizeVersion, ver)
	}

	return nil
}

func checkFuryagentVersion(ver string) error {
	if ver == "" {
		return nil
	}

	out, err := execCommand("furyagent", "version").Output()
	if err != nil {
		return err
	}

	s := string(out)

	pattern := regexp.MustCompile("version (\\S*)")

	versionStringIndex := pattern.FindStringIndex(s)
	if versionStringIndex == nil {
		return fmt.Errorf("can't get furyagent version from system")
	}

	versionString := s[versionStringIndex[0]:versionStringIndex[1]]

	versionStringTokens := strings.Split(versionString, " ")
	if len(versionStringTokens) == 0 {
		return fmt.Errorf("can't get furyagent version from system")
	}

	systemFuryagentVersion := versionStringTokens[len(versionStringTokens)-1]

	if systemFuryagentVersion != ver {
		return fmt.Errorf("furyagent version on system: %s, required version: %s", systemFuryagentVersion, ver)
	}

	return nil
}
