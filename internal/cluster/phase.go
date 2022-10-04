// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

const (
	CreationPhaseInfrastructure = "infrastructure"
	CreationPhaseKubernetes     = "kubernetes"
	CreationPhaseDistribution   = "distribution"
	CreationPhaseAll            = ""

	CreationPhaseOptionVPNAutoConnect = "vpnautoconnect"
)

type CreationPhaseOption struct {
	Name  string
	Value any
}

type CreationPhase interface {
	Exec(dryRun bool, opts []CreationPhaseOption)
}
