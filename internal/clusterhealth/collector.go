// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterhealth

import (
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

// Collector reads cluster state through kubectl. Every read is a plain get; nothing is
// modified.
type Collector struct {
	runner *kubectl.Runner
}

func NewCollector(kubectlBin, workDir string) *Collector {
	return &Collector{
		runner: kubectl.NewRunner(
			execx.NewStdExecutor(),
			kubectl.Paths{Kubectl: kubectlBin, WorkDir: workDir},
			false, true, false,
		),
	}
}

// Collect runs every health check. A check that cannot run records why, and the report
// keeps going: a partial picture is still worth having, as long as it says which parts are
// missing.
func (c *Collector) Collect() *Report {
	report := &Report{}

	podsJSON, podsErr := c.runner.Get(false, "all", "pods", "-o", "json")
	nodesJSON, nodesErr := c.runner.Get(false, "all", "nodes", "-o", "json")

	report.Checks = append(report.Checks,
		check("pods", "pods that are not Running or Succeeded, and unusual restart counts",
			podsErr, func() ([]Issue, error) { return parsePods(podsJSON) }),
		c.workloadCheck(),
		c.disruptionBudgetCheck(),
		c.usageCheck(),
		check("drain-fit", "whether each node's pod requests would fit on its peers during a drain",
			firstErr(nodesErr, podsErr), func() ([]Issue, error) {
				capacities, err := parseCapacity(nodesJSON, podsJSON)
				if err != nil {
					return nil, err
				}

				return DrainFit(capacities), nil
			}),
	)

	return report
}

func (c *Collector) workloadCheck() Check {
	raw, err := c.runner.Get(false, "all", "deployments,statefulsets,daemonsets", "-o", "json")

	return check("workloads", "Deployments, StatefulSets and DaemonSets with missing replicas",
		err, func() ([]Issue, error) { return parseWorkloads(raw) })
}

func (c *Collector) disruptionBudgetCheck() Check {
	raw, err := c.runner.Get(false, "all", "poddisruptionbudgets", "-o", "json")

	return check("disruption-budgets", "disruption budgets that would block a node drain",
		err, func() ([]Issue, error) { return parsePDBs(raw) })
}

func (c *Collector) usageCheck() Check {
	raw, err := c.runner.Top("nodes", "--no-headers")

	return check("node-usage", "nodes close to memory saturation",
		err, func() ([]Issue, error) {
			usage, parseErr := parseTopNodes(raw)
			if parseErr != nil {
				return nil, parseErr
			}

			return parseUsage(usage), nil
		})
}

// check runs one parser, recording a failure rather than letting it look like a clean pass.
func check(name, description string, fetchErr error, parse func() ([]Issue, error)) Check {
	result := Check{Name: name, Description: description}

	if fetchErr != nil {
		result.Err = fetchErr.Error()

		return result
	}

	issues, err := parse()
	if err != nil {
		result.Err = err.Error()

		return result
	}

	result.Issues = issues

	return result
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
