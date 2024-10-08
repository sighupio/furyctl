// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kfddistribution

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/parser"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var ErrKubeconfigNotSet = errors.New("KUBECONFIG env variable is not set")

type KubeconfigGetter struct {
	furyctlConf public.KfddistributionKfdV1Alpha2
	kfdManifest config.KFD
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
		if s, ok := value.(public.KfddistributionKfdV1Alpha2); ok {
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

	case cluster.KubeconfigPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			k.kfdManifest = s
		}
	}
}

func (k *KubeconfigGetter) Get() error {
	logrus.Info("Getting kubeconfig...")

	var err error

	cfgParser := parser.NewConfigParser(k.configPath)

	kubeconfigPath := os.Getenv("KUBECONFIG")

	if distribution.HasFeature(k.kfdManifest, distribution.FeatureKubeconfigInSchema) {
		kubeconfigPath, err = cfgParser.ParseDynamicValue(k.furyctlConf.Spec.Distribution.Kubeconfig)
		if err != nil {
			return fmt.Errorf("error parsing kubeconfig value: %w", err)
		}
	} else if kubeconfigPath == "" {
		return ErrKubeconfigNotSet
	}

	kubeconfig, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error reading kubeconfig file: %w", err)
	}

	kubeconfigPath = path.Join(k.workDir, "kubeconfig")

	if err := os.WriteFile(kubeconfigPath, kubeconfig, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return nil
}
