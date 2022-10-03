// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/yaml"
)

var factories = make(map[string]map[string]CreatorFactory)

type CreatorFactory func(configPath string, props []CreatorProperty) (Creator, error)

type CreatorProperty struct {
	Name  string
	Value any
}

type Creator interface {
	SetProperties(props []CreatorProperty)
	SetProperty(name string, value any)
	Create(dryRun bool) error
	Infrastructure(dryRun bool) error
	Kubernetes(dryRun bool) error
	Distribution(dryRun bool) error
}

func NewCreator(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	configPath string,
	phase string,
	vpnAutoConnect bool,
) (Creator, error) {
	lcApiVersion := strings.ToLower(minimalConf.ApiVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := factories[lcApiVersion][lcResourceType]; ok {
		return factoryFn(configPath, []CreatorProperty{
			{
				Name:  "kfdManifest",
				Value: kfdManifest,
			},
			{
				Name:  "phase",
				Value: phase,
			},
			{
				Name:  "vpnAutoConnect",
				Value: vpnAutoConnect,
			},
		})
	}

	return nil, fmt.Errorf("resource type '%s' with api version '%s' is not supported", lcResourceType, lcApiVersion)
}

func RegisterCreatorFactory(apiVersion, kind string, factory CreatorFactory) {
	lcApiVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := factories[lcApiVersion]; !ok {
		factories[lcApiVersion] = make(map[string]CreatorFactory)
	}

	factories[lcApiVersion][lcKind] = factory
}

func NewCreatorFactory[T Creator, S any]() CreatorFactory {
	return func(configPath string, props []CreatorProperty) (Creator, error) {
		var cc T

		furyctlConf, err := yaml.FromFileV3[S](configPath)
		if err != nil {
			return nil, err
		}

		cc.SetProperty("configPath", configPath)
		cc.SetProperty("furyctlConf", furyctlConf)
		cc.SetProperties(props)

		return cc, nil
	}
}
