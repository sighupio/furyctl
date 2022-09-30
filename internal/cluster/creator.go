// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
)

var factories = make(map[string]map[string]CreatorFactory)

type CreatorFactory func(opts []CreatorOption[any]) Creator

type CreatorOption[T any] struct {
	Name  string
	Value T
}

type Creator interface {
	SetOptions(opt []CreatorOption[any])
	SetOption(opt CreatorOption[any])
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
		return factoryFn([]CreatorOption[any]{
			{
				Name:  "kfdManifest",
				Value: kfdManifest,
			},
			{
				Name:  "configPath",
				Value: configPath,
			},
			{
				Name:  "phase",
				Value: phase,
			},
			{
				Name:  "vpnAutoConnect",
				Value: vpnAutoConnect,
			},
		}), nil
	}

	return nil, fmt.Errorf("resource type '%s' with api version '%s' is not supported", lcResourceType, lcApiVersion)
}

func RegisterCreatorFactory(apiVersion string, kind string, factory func(opts []CreatorOption[any]) Creator) {
	if _, ok := factories[apiVersion]; !ok {
		factories[apiVersion] = make(map[string]CreatorFactory)
	}

	factories[apiVersion][kind] = factory
}
