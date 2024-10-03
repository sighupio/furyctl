// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/awscli"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type KubeconfigGetter struct {
	furyctlConf private.EksclusterKfdV1Alpha2
	configPath  string
	workDir     string
}

func (k *KubeconfigGetter) SetProperties(props []cluster.KubeconfigProperty) {
	for _, prop := range props {
		k.SetProperty(prop.Name, prop.Value)
	}
}

func (k *KubeconfigGetter) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.KubeconfigPropertyFuryctlConf:
		if s, ok := value.(private.EksclusterKfdV1Alpha2); ok {
			k.furyctlConf = s
		}

	case cluster.KubeconfigPropertyConfigPath:
		if s, ok := value.(string); ok {
			k.configPath = s
		}

	case cluster.KubeconfigPropertyWorkDir:
		if s, ok := value.(string); ok {
			k.workDir = s
		}
	}
}

func (k *KubeconfigGetter) Get() error {
	logrus.Info("Getting kubeconfig...")

	kubeconfigPath := path.Join(k.workDir, "kubeconfig")

	awsRunner := awscli.NewRunner(
		execx.NewStdExecutor(),
		awscli.Paths{
			Awscli:  "aws",
			WorkDir: k.workDir,
		},
	)

	if _, err := awsRunner.Eks(
		false,
		"update-kubeconfig",
		"--name",
		k.furyctlConf.Metadata.Name,
		"--kubeconfig",
		kubeconfigPath,
		"--region",
		string(k.furyctlConf.Spec.Region),
	); err != nil {
		return fmt.Errorf("error getting kubeconfig: %w", err)
	}

	return nil
}
