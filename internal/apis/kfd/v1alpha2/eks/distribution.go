// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
)

type Distribution struct {
	base        *Base
	furyctlConf schema.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
}

func NewDistribution(furyctlConf schema.EksclusterKfdV1Alpha2, kfdManifest config.KFD) (*Distribution, error) {
	base, err := NewBase(".distribution")
	if err != nil {
		return nil, err
	}

	return &Distribution{
		base:        base,
		furyctlConf: furyctlConf,
		kfdManifest: kfdManifest,
	}, nil
}

func (d *Distribution) Exec(dryRun bool) error {
	return d.base.CreateFolder()
}
