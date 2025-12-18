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

type CertificatesRenewer struct {
	*cluster.OperationPhase
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
	configPath  string
}

func (c *CertificatesRenewer) SetProperties(props []cluster.CertificatesRenewerProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}

	c.OperationPhase = &cluster.OperationPhase{}
}

func (c *CertificatesRenewer) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CertificatesRenewerPropertyFuryctlConf:
		if s, ok := value.(public.ImmutableKfdV1Alpha2); ok {
			c.furyctlConf = s
		}

	case cluster.CertificatesRenewerPropertyConfigPath:
		if s, ok := value.(string); ok {
			c.configPath = s
		}

	case cluster.CertificatesRenewerPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			c.kfdManifest = s
		}

	case cluster.CertificatesRenewerPropertyDistroPath:
		if s, ok := value.(string); ok {
			c.distroPath = s
		}
	}
}

func (c *CertificatesRenewer) Renew() error {
	return fmt.Errorf("certificates renewal not implemented for Immutable kind")
}
