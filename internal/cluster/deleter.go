// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	DeleterPropertyConfigPath     = "configpath"
	DeleterPropertyFuryctlConf    = "furyctlconf"
	DeleterPropertyPhase          = "phase"
	DeleterPropertyWorkDir        = "workdir"
	DeleterPropertyKfdManifest    = "kfdmanifest"
	DeleterPropertyDistroPath     = "distropath"
	DeleterPropertyBinPath        = "binpath"
	DeleterPropertySkipVpn        = "skipvpn"
	DeleterPropertyVpnAutoConnect = "vpnautoconnect"
	DeleterPropertyDryRun         = "dryrun"
)

var delFactories = make(map[string]map[string]DeleterFactory) //nolint:gochecknoglobals, lll // This patterns requires factories
//  as global to work with init function.

type DeleterPaths struct {
	DistroPath string
	ConfigPath string
	WorkDir    string
	BinPath    string
}

type DeleterFactory func(configPath string, props []DeleterProperty) (Deleter, error)

type DeleterProperty struct {
	Name  string
	Value any
}

type Deleter interface {
	SetProperties(props []DeleterProperty)
	SetProperty(name string, value any)
	Delete() error
}

func NewDeleter(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	paths DeleterPaths,
	phase string,
	skipVpn,
	vpnAutoConnect,
	dryRun bool,
) (Deleter, error) {
	if err := resetKubeconfigEnv(kfdManifest); err != nil {
		return nil, fmt.Errorf("error resetting kubeconfig env: %w", err)
	}

	lcAPIVersion := strings.ToLower(minimalConf.APIVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := delFactories[lcAPIVersion][lcResourceType]; ok {
		return factoryFn(paths.ConfigPath, []DeleterProperty{
			{
				Name:  DeleterPropertyKfdManifest,
				Value: kfdManifest,
			},
			{
				Name:  DeleterPropertyPhase,
				Value: phase,
			},
			{
				Name:  DeleterPropertyWorkDir,
				Value: paths.WorkDir,
			},
			{
				Name:  DeleterPropertyDistroPath,
				Value: paths.DistroPath,
			},
			{
				Name:  DeleterPropertyBinPath,
				Value: paths.BinPath,
			},
			{
				Name:  DeleterPropertySkipVpn,
				Value: skipVpn,
			},
			{
				Name:  DeleterPropertyVpnAutoConnect,
				Value: vpnAutoConnect,
			},
			{
				Name:  DeleterPropertyDryRun,
				Value: dryRun,
			},
		})
	}

	return nil, fmt.Errorf("%w -  type '%s' api version '%s'", errResourceNotSupported, lcResourceType, lcAPIVersion)
}

func RegisterDeleterFactory(apiVersion, kind string, factory DeleterFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := delFactories[lcAPIVersion]; !ok {
		delFactories[lcAPIVersion] = make(map[string]DeleterFactory)
	}

	delFactories[lcAPIVersion][lcKind] = factory
}

func NewDeleterFactory[T Deleter, S any](dd T) DeleterFactory {
	return func(configPath string, props []DeleterProperty) (Deleter, error) {
		furyctlConf, err := yamlx.FromFileV3[S](configPath)
		if err != nil {
			return nil, err
		}

		dd.SetProperty(DeleterPropertyConfigPath, configPath)
		dd.SetProperty(DeleterPropertyFuryctlConf, furyctlConf)
		dd.SetProperties(props)

		return dd, nil
	}
}
