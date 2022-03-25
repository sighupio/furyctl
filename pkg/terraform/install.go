// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-checkpoint"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/hashicorp/terraform-exec/tfinstall"
	"github.com/sighupio/furyctl/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// ensure ensures a working terraform version to be used in the project
func ensure(terraformVersion string, terraformDownloadPath string) (binPath string, err error) {
	if terraformVersion != "" {
		log.Debugf("Installing terraform %v version", terraformVersion)
		return install(terraformVersion, terraformDownloadPath)
	}
	log.Debug("Installing terraform latest version")
	return installLatest(terraformDownloadPath)
}

func alreadyAvailable(terraformVersion string, terraformDownloadPath string) (bool, string) {
	// validate version
	v, err := version.NewVersion(terraformVersion)
	if err != nil {
		log.Warning(err)
		return false, ""
	}
	expectedTerraformBinary := filepath.Join(terraformDownloadPath, "terraform")
	binPath, err := tfinstall.Find(context.Background(), tfinstall.ExactPath(expectedTerraformBinary))
	if err != nil {
		defer os.RemoveAll(expectedTerraformBinary)
		defer os.RemoveAll(binPath)
		log.Warning(err)
		return false, ""
	}
	wd, err := ioutil.TempDir("", "tfexec")
	if err != nil {
		defer os.RemoveAll(expectedTerraformBinary)
		defer os.RemoveAll(binPath)
		log.Warning(err)
		return false, ""
	}
	defer os.RemoveAll(wd) // Clean up
	tf, err := tfexec.NewTerraform(wd, binPath)
	if err != nil {
		defer os.RemoveAll(expectedTerraformBinary)
		defer os.RemoveAll(binPath)
		log.Warning(err)
		return false, ""
	}
	installedV, _, err := tf.Version(context.Background(), true)
	if err != nil {
		defer os.RemoveAll(expectedTerraformBinary)
		defer os.RemoveAll(binPath)
		log.Warning(err)
		return false, ""
	}
	if !v.Equal(installedV) {
		log.Warning("The installed version is different to the required version")
		log.Debug("Removing old terraform version")
		defer os.RemoveAll(expectedTerraformBinary)
		defer os.RemoveAll(binPath)
		return false, ""
	}
	log.Debugf("%s is up to date with the requested %s version", binPath, terraformVersion)
	log.Info("terraform is up to date")
	return true, binPath
}

func install(terraformVersion string, terraformDownloadPath string) (binPath string, err error) {
	ready, binPath := alreadyAvailable(terraformVersion, terraformDownloadPath)
	if !ready {
		err := utils.EnsureDir(filepath.Join(terraformDownloadPath, "terraform"))
		if err != nil {
			return "", err
		}
		binPath, err = tfinstall.Find(context.Background(), tfinstall.ExactVersion(terraformVersion, terraformDownloadPath))
		if err != nil {
			log.Errorf("Error downloading version %v: %v", terraformVersion, err)
			return "", err
		}
	}
	return binPath, nil
}

func installLatest(terraformDownloadPath string) (binPath string, err error) {
	terraformVersion, err := latestVersion(true)
	if err != nil {
		return "", err
	}
	ready, binPath := alreadyAvailable(terraformVersion, terraformDownloadPath)
	if !ready {
		err := utils.EnsureDir(filepath.Join(terraformDownloadPath, "terraform"))
		if err != nil {
			return "", err
		}
		binPath, err = tfinstall.Find(context.Background(), tfinstall.LatestVersion(terraformDownloadPath, false))
		if err != nil {
			log.Errorf("Error downloading latest version: %v", err)
			return "", err
		}
	}
	return binPath, nil
}

func latestVersion(forceCheckpoint bool) (string, error) {
	resp, err := checkpoint.Check(&checkpoint.CheckParams{
		Product: "terraform",
		Force:   forceCheckpoint,
	})
	if err != nil {
		return "", err
	}

	if resp.CurrentVersion == "" {
		return "", fmt.Errorf("could not determine latest version of terraform using checkpoint: CHECKPOINT_DISABLE may be set")
	}

	return resp.CurrentVersion, nil
}
