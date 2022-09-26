// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"errors"
	"github.com/sighupio/fury-distribution/pkg/schemas"
	"github.com/sighupio/furyctl/internal/yaml"

	"github.com/sighupio/furyctl/internal/distribution"
)

var ErrUnsupportedApiVersion = errors.New("unsupported api version")

type ClusterCreator interface {
	Create(dryRun bool) error
}

func NewClusterCreator(
	apiVersion string,
	phase string,
	kfdManifest distribution.Manifest,
	configPath string,
	vpnAutoConnect bool,
) (ClusterCreator, error) {
	switch apiVersion {
	case "kfd.sighup.io/v1alpha2":
		furyFile, err := yaml.FromFileV3[schemas.EksclusterKfdV1Alpha2Json](configPath)
		if err != nil {
			return nil, err
		}

		return &V1alpha2{
			Phase:          phase,
			KfdManifest:    kfdManifest,
			FuryFile:       furyFile,
			ConfigPath:     configPath,
			VpnAutoConnect: vpnAutoConnect,
		}, nil
	}

	return nil, ErrUnsupportedApiVersion
}
