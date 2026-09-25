// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package clusterhealth reads the state of a running cluster and reports what would get in
// the way of an upgrade: workloads that are not healthy, nodes under pressure, disruption
// budgets that block a drain, and nodes whose workload would not fit elsewhere.
//
// Every check carries whether it could run. A check that failed produces no issues, exactly
// like a check that passed, and reporting the two the same way would turn a broken lookup
// into a clean bill of health.
package clusterhealth

// Severity ranks an issue by what it demands before an upgrade starts.
const (
	// SeverityBlocker marks something that will make the upgrade fail or hang.
	SeverityBlocker = "blocker"
	// SeverityWarning marks something to look at, which does not by itself stop the upgrade.
	SeverityWarning = "warning"
)

// Report is the outcome of every health check.
type Report struct {
	Checks []Check `json:"checks" yaml:"checks"`
}

// Blockers returns every issue that must be resolved before upgrading.
func (r *Report) Blockers() []Issue {
	var blockers []Issue

	for _, check := range r.Checks {
		for _, issue := range check.Issues {
			if issue.Severity == SeverityBlocker {
				blockers = append(blockers, issue)
			}
		}
	}

	return blockers
}

// Failed returns the checks that could not run, which is not the same as passing.
func (r *Report) Failed() []Check {
	var failed []Check

	for _, check := range r.Checks {
		if check.Err != "" {
			failed = append(failed, check)
		}
	}

	return failed
}

// Check is one health check.
type Check struct {
	// Name identifies the check, for example "pods" or "drain-fit".
	Name string `json:"name" yaml:"name"`
	// Description says what the check looked at, so a clean result is meaningful.
	Description string `json:"description" yaml:"description"`
	// Err is set when the check could not run. Issues is then meaningless, not empty.
	Err string `json:"err,omitempty" yaml:"err,omitempty"`
	// Issues is what the check found. Empty with no Err means the cluster is clean for it.
	Issues []Issue `json:"issues,omitempty" yaml:"issues,omitempty"`
}

// Ran reports whether the check produced a usable answer.
func (c *Check) Ran() bool {
	return c.Err == ""
}

// Clean reports whether the check ran and found nothing. It is deliberately false for a
// check that failed.
func (c *Check) Clean() bool {
	return c.Ran() && len(c.Issues) == 0
}

// Issue is one thing a check found.
type Issue struct {
	Severity string `json:"severity"         yaml:"severity"`
	Subject  string `json:"subject"          yaml:"subject"`
	Detail   string `json:"detail,omitempty" yaml:"detail,omitempty"`
}
