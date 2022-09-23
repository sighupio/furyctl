// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"errors"

	"github.com/sighupio/furyctl/internal/distribution"
)

var ErrUnsupportedApiVersion = errors.New("unsupported api version")

type ClusterCreator interface {
	WithPhase(phase string) ClusterCreator
	WithKfdManifest(kfdManifest distribution.Manifest) ClusterCreator
	WithConfigPath(configPath string) ClusterCreator
	WithVpnAutoConnect(vpnAutoConnect bool) ClusterCreator
	Create() error
}

func NewClusterCreator(apiVersion string) (ClusterCreator, error) {
	switch apiVersion {
	case "kfd.sighup.io/v1alpha2":
		return &V1alpha2{}, nil
	}

	return nil, ErrUnsupportedApiVersion
}
