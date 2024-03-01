// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package supported

import (
	"slices"

	"github.com/sighupio/furyctl/internal/cluster"
)

type Phases struct{}

func NewPhases() *Phases {
	return &Phases{}
}

func (*Phases) Get() []string {
	return []string{
		cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseDistribution,
		cluster.OperationPhasePlugins,
	}
}

func (s *Phases) IsSupported(phase string) bool {
	return slices.Contains(s.Get(), phase)
}
