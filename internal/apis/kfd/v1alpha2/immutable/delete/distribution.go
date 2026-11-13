// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package del deletes the Immutable cluster workloads by phase.
package del

import (
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// GracePeriod bounds each kapp pass: a cluster being deleted is broken or decommissioned.
	GracePeriod = 15 * time.Minute

	distributionApp = "kfd"
)

//nolint:gochecknoglobals // Constant values that Go cannot declare as const.
var (
	// The kapp duration form of GracePeriod, "15m".
	graceFlag = strconv.Itoa(int(GracePeriod.Minutes())) + "m"

	// The tigera objects go in a last pass: tigera removes the CNI that terminating pods need.
	calicoExclusion = exclusion{
		{Kinds: []string{"Installation", "APIServer", "Goldmane", "Whisker"}},
		{Namespaces: []string{"tigera-operator", "calico-system"}},
		{Names: []string{
			"tigera-operator%", "calico-system", "%.operator.tigera.io", "%.projectcalico.org", "%.policy.networking.k8s.io",
		}},
	}
)

// KappRunner is the part of kapp.Runner that the delete uses.
type KappRunner interface {
	Delete(app string, params ...string) error
	Inspect(app string, params ...string) (string, error)
}

// kappDeleter runs the kapp deletes of the distribution and of the kustomize plugins with the same params.
type kappDeleter struct {
	kapp   KappRunner
	dryRun bool
}

type Distribution struct {
	kappDeleter

	networking string
}

func NewDistribution(kapp KappRunner, networking string, dryRun bool) *Distribution {
	return &Distribution{kappDeleter: kappDeleter{kapp: kapp, dryRun: dryRun}, networking: networking}
}

// Exec deletes the kapp app kfd, relying on kapp's order: custom resources, then CRDs, then the rest.
func (d *Distribution) Exec() error {
	if d.networking == "calico" {
		logrus.Info("Deleting the distribution, calico excluded...")

		if err := d.deleteApp(distributionApp, calicoExclusion); err != nil {
			return err
		}

		logrus.Info("Deleting calico...")
	} else {
		logrus.Info("Deleting the distribution...")
	}

	return d.deleteApp(distributionApp, nil)
}

func (k kappDeleter) deleteApp(app string, skip exclusion) error {
	return deleteApp(k.kapp, app, k.params(), skip)
}

func (k kappDeleter) params() []string {
	// --apply-ignored: kapp waits for the PVCs of StatefulSets without deleting them unless asked to.
	params := []string{
		"--apply-ignored",
		"--apply-concurrency", "20",
		"--apply-timeout", graceFlag,
		"--wait-timeout", graceFlag,
	}

	if k.dryRun {
		params = append(params, "--diff-run")
	}

	return params
}
