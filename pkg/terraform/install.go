// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"context"

	"github.com/hashicorp/terraform-exec/tfinstall"
	log "github.com/sirupsen/logrus"
)

// ensure ensures a working terraform version to be used in the project
func ensure(terraformBinaryPath string, terraformVersion string, terraformDownloadPath string) (binPath string, err error) {
	if terraformBinaryPath != "" {
		log.Debugf("Check if %v the terraform binary path is valid", terraformBinaryPath)
		return checkBinary(terraformBinaryPath)
	}
	if terraformVersion != "" {
		log.Debugf("Installing terraform %v version", terraformVersion)
		return install(terraformVersion, terraformDownloadPath)
	}
	log.Debug("Installing terraform latest version")
	return installLatest(terraformDownloadPath)
}

func checkBinary(terraformBinaryPath string) (binPath string, err error) {
	binPath, err = tfinstall.Find(context.Background(), tfinstall.ExactPath(terraformBinaryPath))
	if err != nil {
		log.Errorf("Terraform binary not found %v", err)
		return "", err
	}
	return binPath, nil
}

func install(terraformVersion string, terraformDownloadPath string) (binPath string, err error) {
	binPath, err = tfinstall.Find(context.Background(), tfinstall.ExactVersion(terraformVersion, terraformDownloadPath))
	if err != nil {
		log.Errorf("Error downloading version %v: %v", terraformVersion, err)
		return "", err
	}
	return binPath, nil
}

func installLatest(terraformDownloadPath string) (binPath string, err error) {
	binPath, err = tfinstall.Find(context.Background(), tfinstall.LatestVersion(terraformDownloadPath, false))
	if err != nil {
		log.Errorf("Error downloading latest version: %v", err)
		return "", err
	}
	return binPath, nil
}
