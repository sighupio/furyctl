// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
)

type Distribution struct {
	*cluster.CreationPhase
	furyctlConf schema.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
}

func NewDistribution(furyctlConf schema.EksclusterKfdV1Alpha2, kfdManifest config.KFD) (*Distribution, error) {
	phase, err := cluster.NewCreationPhase(".distribution")
	if err != nil {
		return nil, err
	}

	return &Distribution{
		CreationPhase: phase,
		furyctlConf:   furyctlConf,
		kfdManifest:   kfdManifest,
	}, nil
}

func (d *Distribution) Exec(dryRun bool) error {
	return d.CreationPhase.CreateFolder()
}
