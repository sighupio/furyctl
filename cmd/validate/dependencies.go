// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/cobrax"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/yaml"
)

var (
	ErrSystemDepsValidation      = errors.New("error while validating system dependencies")
	ErrEnvironmentDepsValidation = errors.New("error while validating environment dependencies")
	ErrEmptyToolVersion          = errors.New("empty tool version")

	execCommand = exec.Command
)

func NewDependenciesCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Validate furyctl.yaml file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			debug := cobrax.Flag[bool](cmd, "debug").(bool)
			binPath := cobrax.Flag[string](cmd, "bin-path").(string)
			furyctlPath := cobrax.Flag[string](cmd, "config").(string)
			distroLocation := cobrax.Flag[string](cmd, "distro-location").(string)

			minimalConf, err := yaml.FromFileV3[distribution.FuryctlConfig](furyctlPath)
			if err != nil {
				return err
			}

			furyctlConfVersion := minimalConf.Spec.DistributionVersion

			if version != "dev" {
				furyctlBinVersion, err := semver.NewVersion(fmt.Sprintf("v%s", version))
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

			if !semver.SamePatch(furyctlConfVersion, kfdManifest.Version) {
				return fmt.Errorf(
					"versions mismatch: furyctl.yaml has %s, but furyctl has %s",
					furyctlConfVersion.String(),
					kfdManifest.Version.String(),
				)
			}

			logrus.Debugln("Checking system dependencies")
			if err := validateSystemDependencies(kfdManifest, binPath); err != nil {
				return err
			}

			logrus.Debugln("Checking environment dependencies")
			if err := validateEnvDependencies(minimalConf.Kind); err != nil {
				return err
			}

			fmt.Println("All dependencies are satisfied")

			return nil
		},
	}

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the bin folder where all dependencies are installed",
	)

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

func validateEnvDependencies(kind distribution.Kind) error {
	errs := make([]error, 0)

	if kind.Equals(distribution.EKSCluster) {
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

func validateSystemDependencies(kfdManifest distribution.Manifest, binPath string) error {
	errs := make([]error, 0)

	if err := checkAnsibleVersion(kfdManifest.Tools.Ansible, binPath); err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	if err := checkTerraformVersion(kfdManifest.Tools.Terraform, binPath); err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	if err := checkKubectlVersion(kfdManifest.Tools.Kubectl, binPath); err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	if err := checkKustomizeVersion(kfdManifest.Tools.Kustomize, binPath); err != nil {
		logrus.Error(err)
		errs = append(errs, err)
	}

	if kfdManifest.Tools.Furyagent != "" {
		if err := checkFuryagentVersion(kfdManifest.Tools.Furyagent, binPath); err != nil {
			logrus.Error(err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return ErrSystemDepsValidation
	}

	return nil
}

func checkAnsibleVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("ansible: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "ansible")
	out, err := execCommand(path, "--version").Output()
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

	if systemAnsibleVersion != wantVer {
		return fmt.Errorf("ansible version on system: %s, required version: %s", systemAnsibleVersion, wantVer)
	}

	return nil
}

func checkTerraformVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("terraform: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "terraform")
	out, err := execCommand(path, "--version").Output()
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

	if systemTerraformVersion != wantVer {
		return fmt.Errorf("terraform version on system: %s, required version: %s", systemTerraformVersion, wantVer)
	}

	return nil
}

func checkKubectlVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("kubectl: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "kubectl")
	out, err := execCommand(path, "version", "--client").Output()
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

	if systemKubectlVersion != wantVer {
		return fmt.Errorf("kubectl version on system: %s, required version: %s", systemKubectlVersion, wantVer)
	}

	return nil
}

func checkKustomizeVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("kustomize: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "kustomize")
	out, err := execCommand(path, "version", "--short").Output()
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

	if systemKustomizeVersion != wantVer {
		return fmt.Errorf("kustomize version on system: %s, required version: %s", systemKustomizeVersion, wantVer)
	}

	return nil
}

func checkFuryagentVersion(wantVer, binPath string) error {
	if wantVer == "" {
		return fmt.Errorf("furyagent: %w", ErrEmptyToolVersion)
	}

	path := filepath.Join(binPath, "furyagent")
	out, err := execCommand(path, "version").Output()
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

	if systemFuryagentVersion != wantVer {
		return fmt.Errorf("furyagent version on system: %s, required version: %s", systemFuryagentVersion, wantVer)
	}

	return nil
}
