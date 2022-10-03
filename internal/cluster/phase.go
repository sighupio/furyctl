// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

const (
	PhaseInfrastructure = "infrastructure"
	PhaseKubernetes     = "kubernetes"
	PhaseDistribution   = "distribution"
	PhaseAll            = ""

	PhaseOptionVPNAutoConnect = "vpnautoconnect"
)

type PhaseOption struct {
	Name  string
	Value any
}

type Phase interface {
	Exec(dryRun bool, opts []PhaseOption)
}
