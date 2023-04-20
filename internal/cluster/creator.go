// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	CreatorPropertyConfigPath     = "configpath"
	CreatorPropertyWorkDir        = "workdir"
	CreatorPropertyFuryctlConf    = "furyctlconf"
	CreatorPropertyKfdManifest    = "kfdmanifest"
	CreatorPropertyDistroPath     = "distropath"
	CreatorPropertyBinPath        = "binpath"
	CreatorPropertyPhase          = "phase"
	CreatorPropertySkipVpn        = "skipvpn"
	CreatorPropertyVpnAutoConnect = "vpnautoconnect"
	CreatorPropertyKubeconfig     = "kubeconfig"
	CreatorPropertyDryRun         = "dryrun"
)

var (
	crFactories = make(map[string]map[string]CreatorFactory) //nolint:gochecknoglobals, lll // This patterns requires crFactories
	//  as global to work with init function.
	errResourceNotSupported = errors.New("resource is not supported")
)

type CreatorPaths struct {
	ConfigPath string
	WorkDir    string
	DistroPath string
	BinPath    string
	Kubeconfig string
}

type CreatorFactory func(configPath string, props []CreatorProperty) (Creator, error)

type CreatorProperty struct {
	Name  string
	Value any
}

type Creator interface {
	SetProperties(props []CreatorProperty)
	SetProperty(name string, value any)
	Create(skipPhase string, timeout int) error
}

func NewCreator(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	paths CreatorPaths,
	phase string,
	skipVpn,
	vpnAutoConnect,
	dryRun bool,
) (Creator, error) {
	lcAPIVersion := strings.ToLower(minimalConf.APIVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := crFactories[lcAPIVersion][lcResourceType]; ok {
		return factoryFn(paths.ConfigPath, []CreatorProperty{
			{
				Name:  CreatorPropertyKfdManifest,
				Value: kfdManifest,
			},
			{
				Name:  CreatorPropertyPhase,
				Value: phase,
			},
			{
				Name:  CreatorPropertySkipVpn,
				Value: skipVpn,
			},
			{
				Name:  CreatorPropertyVpnAutoConnect,
				Value: vpnAutoConnect,
			},
			{
				Name:  CreatorPropertyDistroPath,
				Value: paths.DistroPath,
			},
			{
				Name:  CreatorPropertyBinPath,
				Value: paths.BinPath,
			},
			{
				Name:  CreatorPropertyWorkDir,
				Value: paths.WorkDir,
			},
			{
				Name:  CreatorPropertyKubeconfig,
				Value: paths.Kubeconfig,
			},
			{
				Name:  CreatorPropertyDryRun,
				Value: dryRun,
			},
		})
	}

	return nil, fmt.Errorf("%w -  type '%s' api version '%s'", errResourceNotSupported, lcResourceType, lcAPIVersion)
}

func RegisterCreatorFactory(apiVersion, kind string, factory CreatorFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := crFactories[lcAPIVersion]; !ok {
		crFactories[lcAPIVersion] = make(map[string]CreatorFactory)
	}

	crFactories[lcAPIVersion][lcKind] = factory
}

func NewCreatorFactory[T Creator, S any](cc T) CreatorFactory {
	return func(configPath string, props []CreatorProperty) (Creator, error) {
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
