// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterhealth

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	// Container restart count above which a pod is worth a look even when it is running.
	restartThreshold = 20
	// Memory usage above which a node is flagged: a node this full may not survive
	// absorbing another node's pods during a drain.
	memoryPressureThreshold = 80
	// Number of whitespace-separated fields in a `kubectl top nodes` line.
	topNodeFields = 5
)

// parsePods reports pods that are neither Running nor Succeeded, and running pods whose
// containers have restarted an unusual number of times.
func parsePods(raw string) ([]Issue, error) {
	var list podList
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("cannot parse the pod list: %w", err)
	}

	var issues []Issue

	for _, pod := range list.Items {
		subject := pod.Metadata.Namespace + "/" + pod.Metadata.Name

		switch pod.Status.Phase {
		case "Running", "Succeeded":
			if restarts, container := worstRestarts(pod); restarts > restartThreshold {
				issues = append(issues, Issue{
					Severity: SeverityWarning,
					Subject:  subject,
					Detail:   fmt.Sprintf("container %s restarted %d times", container, restarts),
				})
			}

		// Failed, Pending and anything else: the pod is not doing its job, and an upgrade
		// that drains nodes will not improve matters.
		default:
			issues = append(issues, Issue{
				Severity: SeverityBlocker,
				Subject:  subject,
				Detail:   phaseDetail(pod),
			})
		}
	}

	return issues, nil
}

// phaseDetail explains why a pod is not running, preferring the scheduler's own words.
func phaseDetail(pod podItem) string {
	detail := pod.Status.Phase

	if pod.Status.Reason != "" {
		detail += ": " + pod.Status.Reason
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == "PodScheduled" && condition.Status == "False" && condition.Message != "" {
			return detail + " (" + condition.Message + ")"
		}
	}

	return detail
}

func worstRestarts(pod podItem) (int, string) {
	worst, name := 0, ""

	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > worst {
			worst, name = status.RestartCount, status.Name
		}
	}

	return worst, name
}

// parseWorkloads reports Deployments, StatefulSets and DaemonSets with fewer ready replicas
// than they want.
func parseWorkloads(raw string) ([]Issue, error) {
	var list workloadList
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("cannot parse the workload list: %w", err)
	}

	var issues []Issue

	for _, item := range list.Items {
		want, ready := desiredAndReady(item)
		if ready >= want {
			continue
		}

		issues = append(issues, Issue{
			Severity: SeverityBlocker,
			Subject:  fmt.Sprintf("%s %s/%s", item.Kind, item.Metadata.Namespace, item.Metadata.Name),
			Detail:   fmt.Sprintf("%d of %d replicas ready", ready, want),
		})
	}

	return issues, nil
}

// desiredAndReady normalises the replica counts, which DaemonSets report under different
// field names than Deployments and StatefulSets.
func desiredAndReady(item workloadItem) (int, int) {
	if item.Kind == "DaemonSet" {
		return item.Status.DesiredNumberScheduled, item.Status.NumberReady
	}

	want := 1
	if item.Spec.Replicas != nil {
		want = *item.Spec.Replicas
	}

	return want, item.Status.ReadyReplicas
}

// parsePDBs reports disruption budgets that allow no disruption at all. Those block a drain
// indefinitely, which is how a node upgrade hangs with no obvious cause.
func parsePDBs(raw string) ([]Issue, error) {
	var list pdbList
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("cannot parse the disruption budget list: %w", err)
	}

	var issues []Issue

	for _, item := range list.Items {
		if item.Status.DisruptionsAllowed > 0 {
			continue
		}

		issues = append(issues, Issue{
			Severity: SeverityBlocker,
			Subject:  fmt.Sprintf("PodDisruptionBudget %s/%s", item.Metadata.Namespace, item.Metadata.Name),
			Detail: fmt.Sprintf(
				"allows no disruption (%d healthy, %d required), a node drain will not complete",
				item.Status.CurrentHealthy, item.Status.DesiredHealthy,
			),
		})
	}

	return issues, nil
}

// parseTopNodes reads the output of `kubectl top nodes` into a usage percentage per node.
// The command prints a header and then one line per node, with CPU and memory as both an
// absolute value and a percentage.
func parseTopNodes(raw string) (map[string]int, error) {
	usage := map[string]int{}

	for line := range strings.SplitSeq(strings.TrimSpace(raw), "\n") {
		fields := strings.Fields(line)

		if len(fields) < topNodeFields || fields[0] == "NAME" {
			continue
		}

		percent, ok := parsePercent(fields[4])
		if !ok {
			continue
		}

		usage[fields[0]] = percent
	}

	if len(usage) == 0 {
		return nil, ErrNoUsageData
	}

	return usage, nil
}

func parsePercent(field string) (int, bool) {
	trimmed := strings.TrimSuffix(field, "%")
	if trimmed == field {
		return 0, false
	}

	value := 0
	if _, err := fmt.Sscanf(trimmed, "%d", &value); err != nil {
		return 0, false
	}

	return value, true
}

// parseUsage turns node usage percentages into issues for the nodes that are nearly full.
func parseUsage(usage map[string]int) []Issue {
	var issues []Issue

	names := make([]string, 0, len(usage))
	for name := range usage {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		if usage[name] < memoryPressureThreshold {
			continue
		}

		issues = append(issues, Issue{
			Severity: SeverityWarning,
			Subject:  "node " + name,
			Detail:   fmt.Sprintf("memory usage is %d%%, close to saturation", usage[name]),
		})
	}

	return issues
}
