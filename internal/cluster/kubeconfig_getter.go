// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // ignoring duplication linting error
package cluster

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	KubeconfigPropertyOutdir      = "outdir"
	KubeconfigPropertyFuryctlConf = "furyctlconf"
	KubeconfigPropertyConfigPath  = "configpath"
	KubeconfigPropertyKfdManifest = "kfdmanifest"
	KubeconfigPropertyDistroPath  = "distropath"
)

var kbFactories = make(map[string]map[string]KubeconfigFactory) //nolint:gochecknoglobals, lll // This patterns requires kbFactories
//  as global to work with init function.

type KubeconfigFactory func(configPath string, props []KubeconfigProperty) (KubeconfigGetter, error)

type KubeconfigProperty struct {
	Name  string
	Value any
}

type KubeconfigGetter interface {
	SetProperties(props []KubeconfigProperty)
	SetProperty(name string, value any)
	Get() error
}

func NewKubeconfigGetter(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	distroPath string,
	configPath string,
	outDir string,
) (KubeconfigGetter, error) {
	lcAPIVersion := strings.ToLower(minimalConf.APIVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := kbFactories[lcAPIVersion][lcResourceType]; ok {
		return factoryFn(configPath, []KubeconfigProperty{
			{
				Name:  KubeconfigPropertyKfdManifest,
				Value: kfdManifest,
			},
			{
				Name:  KubeconfigPropertyOutdir,
				Value: outDir,
			},
			{
				Name:  KubeconfigPropertyDistroPath,
				Value: distroPath,
			},
		})
	}

	return nil, fmt.Errorf("%w -  type '%s' api version '%s'", errResourceNotSupported, lcResourceType, lcAPIVersion)
}

func RegisterKubeconfigFactory(apiVersion, kind string, factory KubeconfigFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := kbFactories[lcAPIVersion]; !ok {
		kbFactories[lcAPIVersion] = make(map[string]KubeconfigFactory)
	}

	kbFactories[lcAPIVersion][lcKind] = factory
}

func NewKubeconfigFactory[T KubeconfigGetter, S any](cc T) KubeconfigFactory {
	return func(configPath string, props []KubeconfigProperty) (KubeconfigGetter, error) {
		furyctlConf, err := yamlx.FromFileV3[S](configPath)
		if err != nil {
			return nil, err
		}

		cc.SetProperty(KubeconfigPropertyConfigPath, configPath)
		cc.SetProperty(KubeconfigPropertyFuryctlConf, furyctlConf)
		cc.SetProperties(props)

		return cc, nil
	}
}
