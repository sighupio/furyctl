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
	CertificatesRenewerPropertyOutdir      = "outdir"
	CertificatesRenewerPropertyFuryctlConf = "furyctlconf"
	CertificatesRenewerPropertyConfigPath  = "configpath"
	CertificatesRenewerPropertyKfdManifest = "kfdmanifest"
	CertificatesRenewerPropertyDistroPath  = "distropath"
)

var certificatesRenewerFactories = make(map[string]map[string]CertificatesRenewerFactory) //nolint:gochecknoglobals, lll // This patterns requires certificatesRenewerFactories as global to work with init function.

type CertificatesRenewerFactory func(configPath string, props []CertificatesRenewerProperty) (CertificatesRenewer, error) //nolint:lll // This pattern requires CertificatesRenewerFactory as global to work with init function.

type CertificatesRenewerProperty struct {
	Name  string
	Value any
}

type CertificatesRenewer interface {
	SetProperties(props []CertificatesRenewerProperty)
	SetProperty(name string, value any)
	Renew() error
}

func NewCertificatesRenewer(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	distroPath string,
	configPath string,
	outDir string,
) (CertificatesRenewer, error) {
	lcAPIVersion := strings.ToLower(minimalConf.APIVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := certificatesRenewerFactories[lcAPIVersion][lcResourceType]; ok {
		return factoryFn(configPath, []CertificatesRenewerProperty{
			{
				Name:  CertificatesRenewerPropertyKfdManifest,
				Value: kfdManifest,
			},
			{
				Name:  CertificatesRenewerPropertyOutdir,
				Value: outDir,
			},
			{
				Name:  CertificatesRenewerPropertyDistroPath,
				Value: distroPath,
			},
		})
	}

	return nil, fmt.Errorf("%w -  type '%s' api version '%s'", errResourceNotSupported, lcResourceType, lcAPIVersion)
}

func RegisterCertificatesRenewerFactory(apiVersion, kind string, factory CertificatesRenewerFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := certificatesRenewerFactories[lcAPIVersion]; !ok {
		certificatesRenewerFactories[lcAPIVersion] = make(map[string]CertificatesRenewerFactory)
	}

	certificatesRenewerFactories[lcAPIVersion][lcKind] = factory
}

func NewCertificatesRenewerFactory[T CertificatesRenewer, S any](cc T) CertificatesRenewerFactory {
	return func(configPath string, props []CertificatesRenewerProperty) (CertificatesRenewer, error) {
		furyctlConf, err := yamlx.FromFileV3[S](configPath)
		if err != nil {
			return nil, err
		}

		cc.SetProperty(CertificatesRenewerPropertyConfigPath, configPath)
		cc.SetProperty(CertificatesRenewerPropertyFuryctlConf, furyctlConf)
		cc.SetProperties(props)

		return cc, nil
	}
}
