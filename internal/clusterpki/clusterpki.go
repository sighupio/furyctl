// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterpki

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	pki "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
)

type ClusterPKI struct {
	Config
}

// Config represents the configuration for the whole cluster's PKI.
type Config struct {
	Etcd         EtcdConfig         `json:"etcd"`
	ControlPlane ControlPlaneConfig `json:"controlPlane"`
	Path         string             `json:"path"`
	CertConfig   pki.CertConfig     `json:"certConfig"`
}

// EtcdConfig is used to store the path to the several certificates for etcd.
type EtcdConfig struct {
	CertDir            string `json:"certDir"`
	CaCertFilename     string `json:"caCertFilename"`
	CaKeyFilename      string `json:"caKeyFilename"`
	ClientCertFilename string `json:"clientCertFilename"`
	ClientKeyFilename  string `json:"clientKeyFilename"`
}

// ControlPlaneConfig is used to store the path to the several certificates for the control plane.
type ControlPlaneConfig struct {
	CertDir          string `json:"certDir"`
	CaCertFile       string `json:"caCertFilename"`
	CaKeyFile        string `json:"caKeyFilename"`
	SaPubFile        string `json:"saPubFilename"`
	SaKeyFile        string `json:"saKeyFilename"`
	ProxyCaCertFile  string `json:"proxyCaCertFilename"`
	ProxyKeyCertFile string `json:"proxyKeyCertFilename"`
}

func (c *Config) save(files map[string][]byte, dir string) error {
	const (
		permOwnerGroup os.FileMode = 0o770
	)

	basePath := filepath.Join(c.Path, dir)
	logrus.Debugf("checking if target path %s exists before proceeding", basePath)

	_, err := os.Stat(basePath)
	if err == nil {
		logrus.Errorf("path '%s' already exists. Delete the folder first if you want to replace its content", basePath)

		return os.ErrExist
	} else if errors.Is(err, os.ErrNotExist) {
		for filename, file := range files {
			fullPath := filepath.Join(basePath, filename)

			logrus.Debug("processing file ", fullPath)

			if err := os.MkdirAll(basePath, permOwnerGroup); err != nil {
				return fmt.Errorf("got an error trying to create dirs %s: %w", basePath, err)
			}

			if err := os.WriteFile(fullPath, file, permOwnerGroup); err != nil {
				return fmt.Errorf("error while saving file %s: %w", fullPath, err)
			}

			logrus.Debug("file successfully saved:", fullPath)
		}

		logrus.Debug("all files saved successfully")

		return nil
	}

	return fmt.Errorf("unexpected error while checking if path exists: %w", err)
}
