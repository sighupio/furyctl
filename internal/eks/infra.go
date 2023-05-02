// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

type Infra struct {
	criticalTFResources []string
}

func NewInfra() *Infra {
	return &Infra{
		criticalTFResources: []string{"aws_vpc", "aws_subnet"},
	}
}

func (i *Infra) GetCriticalTFResources() []string {
	return i.criticalTFResources
}
