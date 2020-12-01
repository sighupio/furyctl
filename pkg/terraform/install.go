package terraform

import (
	"context"
	"io/ioutil"

	"github.com/hashicorp/terraform-exec/tfinstall"
	log "github.com/sirupsen/logrus"
)

// ensure ensures a working terraform version to be used in the project
func ensure(terraformBinaryPath string, terraformVersion string) (binPath string, err error) {
	if terraformBinaryPath != "" {
		log.Debugf("Check if %v the terraform binary path is valid", terraformBinaryPath)
		return checkBinary(terraformBinaryPath)
	}
	if terraformVersion != "" {
		log.Debugf("Installing terraform %v version", terraformVersion)
		return install(terraformVersion)
	}
	log.Debug("Installing terraform latest version")
	return installLatest()
}

func checkBinary(terraformBinaryPath string) (binPath string, err error) {
	binPath, err = tfinstall.Find(context.Background(), tfinstall.ExactPath(terraformBinaryPath))
	if err != nil {
		log.Errorf("Terraform binary not found %v", err)
		return "", err
	}
	return binPath, nil
}

func install(terraformVersion string) (binPath string, err error) {
	tmpDir, err := ioutil.TempDir("", "tfinstall")
	binPath, err = tfinstall.Find(context.Background(), tfinstall.ExactVersion(terraformVersion, tmpDir))
	if err != nil {
		log.Errorf("Error downloading version %v: %v", terraformVersion, err)
		return "", err
	}
	return binPath, nil
}

func installLatest() (binPath string, err error) {
	tmpDir, err := ioutil.TempDir("", "tfinstall")
	binPath, err = tfinstall.Find(context.Background(), tfinstall.LatestVersion(tmpDir, false))
	if err != nil {
		log.Errorf("Error downloading latest version: %v", err)
		return "", err
	}
	return binPath, nil
}
