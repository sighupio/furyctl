// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	CreatorPropertyConfigPath     = "configpath"
	CreatorPropertyFuryctlConf    = "furyctlconf"
	CreatorPropertyKfdManifest    = "kfdmanifest"
	CreatorPropertyPhase          = "phase"
	CreatorPropertyVpnAutoConnect = "vpnautoconnect"
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
				Name:  CreatorPropertyKfdManifest,
				Value: kfdManifest,
			},
			{
				Name:  CreatorPropertyPhase,
				Value: phase,
			},
			{
				Name:  CreatorPropertyVpnAutoConnect,
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

		furyctlConf, err := yamlx.FromFileV3[S](configPath)
		if err != nil {
			return nil, err
		}

		cc.SetProperty(CreatorPropertyConfigPath, configPath)
		cc.SetProperty(CreatorPropertyFuryctlConf, furyctlConf)
		cc.SetProperties(props)

		return cc, nil
	}
}
