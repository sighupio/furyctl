// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package supported

import (
	"github.com/sighupio/furyctl/internal/cluster"
)

func Phases() cluster.SupportedPhases {
	return cluster.SupportedPhases{
		cluster.OperationPhaseDistribution,
		cluster.OperationPhasePlugins,
	}
}
