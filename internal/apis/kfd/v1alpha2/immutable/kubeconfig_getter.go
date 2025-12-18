// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

type KubeconfigGetter struct {
	*cluster.OperationPhase
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
	workDir     string
}

func (k *KubeconfigGetter) SetProperties(props []cluster.KubeconfigProperty) {
	for _, prop := range props {
		k.SetProperty(prop.Name, prop.Value)
	}

	k.OperationPhase = &cluster.OperationPhase{}
}

func (k *KubeconfigGetter) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.KubeconfigPropertyFuryctlConf:
		if s, ok := value.(public.ImmutableKfdV1Alpha2); ok {
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

	case cluster.KubeconfigPropertyDistroPath:
		if s, ok := value.(string); ok {
			k.distroPath = s
		}
	}
}

func (k *KubeconfigGetter) Get() error {
	return fmt.Errorf("kubeconfig get not implemented for Immutable kind")
}
