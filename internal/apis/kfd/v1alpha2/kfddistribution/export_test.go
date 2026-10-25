// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package kfddistribution

// SetNewPreflight replaces the constructor of the preflight that Diff runs.
func (c *ClusterCreator) SetNewPreflight(f func(dryRun bool, force []string, startFrom string, upgradeEnabled bool) DiffPreflight) {
	c.newPreflight = f
}
