// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/clusterhealth"
	"github.com/sighupio/furyctl/internal/clusterinfo"
)

// ongoingUpgradeCheck reports an upgrade that is already running, which has to finish or be
// rolled back before another one is planned.
func ongoingUpgradeCheck(info *clusterinfo.Info) clusterhealth.Check {
	check := clusterhealth.Check{
		Name:        "ongoing-upgrade",
		Description: "an upgrade already in progress on this cluster",
	}

	if info.SDOngoingUpgradeError != "" {
		check.Err = info.SDOngoingUpgradeError

		return check
	}

	if info.SDOngoingUpgrade == nil {
		return check
	}

	upgrade := info.SDOngoingUpgrade
	detail := fmt.Sprintf("phase %s is %s", upgrade.Phase, upgrade.Status)

	if upgrade.From != "" && upgrade.To != "" {
		detail += fmt.Sprintf(", %s -> %s", upgrade.From, upgrade.To)
	}

	check.Issues = []clusterhealth.Issue{{
		Severity: clusterhealth.SeverityBlocker,
		Subject:  "cluster " + info.ClusterName,
		Detail:   detail + "; finish or roll back that upgrade before planning another",
	}}

	return check
}
