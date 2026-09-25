// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // white-box tests for the parsers
package clusterhealth

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePods(t *testing.T) {
	t.Parallel()

	raw := `{"items":[
      {"metadata":{"name":"ok","namespace":"default"},
       "status":{"phase":"Running","containerStatuses":[{"name":"c","restartCount":2}]}},
      {"metadata":{"name":"done","namespace":"default"},"status":{"phase":"Succeeded"}},
      {"metadata":{"name":"flapping","namespace":"default"},
       "status":{"phase":"Running","containerStatuses":[{"name":"c","restartCount":57}]}},
      {"metadata":{"name":"stuck","namespace":"apps"},
       "status":{"phase":"Pending","conditions":[
         {"type":"PodScheduled","status":"False","message":"0/3 nodes are available: insufficient memory"}]}},
      {"metadata":{"name":"gone","namespace":"apps"},
       "status":{"phase":"Failed","reason":"Evicted"}}
    ]}`

	issues, err := parsePods(raw)
	require.NoError(t, err, "parsePods")
	require.Len(t, issues, 3, "one restart warning, one pending, one failed")

	bySubject := map[string]Issue{}
	for _, issue := range issues {
		bySubject[issue.Subject] = issue
	}

	assert.Equal(t, SeverityWarning, bySubject["default/flapping"].Severity, "restarts are a warning")
	assert.Contains(t, bySubject["default/flapping"].Detail, "57 times", "restart count")

	assert.Equal(t, SeverityBlocker, bySubject["apps/stuck"].Severity, "a pending pod blocks")
	assert.Contains(t, bySubject["apps/stuck"].Detail, "insufficient memory",
		"the scheduler's own explanation is the useful part")

	assert.Equal(t, SeverityBlocker, bySubject["apps/gone"].Severity, "a failed pod blocks")
	assert.Contains(t, bySubject["apps/gone"].Detail, "Evicted", "the reason")

	assert.NotContains(t, bySubject, "default/ok", "a healthy pod is not an issue")
	assert.NotContains(t, bySubject, "default/done", "a completed pod is not an issue")
}

func TestParseWorkloads(t *testing.T) {
	t.Parallel()

	raw := `{"items":[
      {"kind":"Deployment","metadata":{"name":"full","namespace":"a"},
       "spec":{"replicas":3},"status":{"readyReplicas":3}},
      {"kind":"Deployment","metadata":{"name":"degraded","namespace":"a"},
       "spec":{"replicas":3},"status":{"readyReplicas":1}},
      {"kind":"StatefulSet","metadata":{"name":"db","namespace":"b"},
       "spec":{"replicas":1},"status":{"readyReplicas":0}},
      {"kind":"DaemonSet","metadata":{"name":"agent","namespace":"c"},
       "status":{"desiredNumberScheduled":10,"numberReady":10}},
      {"kind":"DaemonSet","metadata":{"name":"partial","namespace":"c"},
       "status":{"desiredNumberScheduled":10,"numberReady":7}}
    ]}`

	issues, err := parseWorkloads(raw)
	require.NoError(t, err, "parseWorkloads")
	require.Len(t, issues, 3, "one deployment, one statefulset, one daemonset")

	subjects := make([]string, 0, len(issues))
	for _, issue := range issues {
		subjects = append(subjects, issue.Subject)
	}

	assert.Contains(t, subjects, "Deployment a/degraded", "degraded deployment")
	assert.Contains(t, subjects, "StatefulSet b/db", "statefulset with no ready replica")
	// DaemonSets count replicas under different field names; getting that wrong would
	// silently report every DaemonSet as broken.
	assert.Contains(t, subjects, "DaemonSet c/partial", "partial daemonset")
	assert.NotContains(t, subjects, "DaemonSet c/agent", "a complete daemonset is fine")
}

func TestParsePDBs(t *testing.T) {
	t.Parallel()

	raw := `{"items":[
      {"metadata":{"name":"ok","namespace":"a"},"status":{"disruptionsAllowed":1}},
      {"metadata":{"name":"blocking","namespace":"a"},
       "status":{"disruptionsAllowed":0,"currentHealthy":2,"desiredHealthy":2}}
    ]}`

	issues, err := parsePDBs(raw)
	require.NoError(t, err, "parsePDBs")
	require.Len(t, issues, 1, "only the budget that allows nothing")

	assert.Equal(t, SeverityBlocker, issues[0].Severity, "a blocked drain is a blocker")
	assert.Contains(t, issues[0].Subject, "a/blocking", "subject")
	assert.Contains(t, issues[0].Detail, "drain will not complete", "why it matters")
}

func TestParseTopNodes(t *testing.T) {
	t.Parallel()

	raw := `worker01   906m   7%    20368Mi   85%
worker02   289m   2%    7580Mi    31%`

	usage, err := parseTopNodes(raw)
	require.NoError(t, err, "parseTopNodes")
	assert.Equal(t, map[string]int{"worker01": 85, "worker02": 31}, usage, "memory percentages")

	issues := parseUsage(usage)
	require.Len(t, issues, 1, "only the saturated node")
	assert.Contains(t, issues[0].Subject, "worker01", "subject")
	assert.Contains(t, issues[0].Detail, "85%", "the figure")
}

func TestParseTopNodesWithoutMetricsServer(t *testing.T) {
	t.Parallel()

	// kubectl prints an error to stderr and nothing usable on stdout.
	_, err := parseTopNodes("error: Metrics API not available")
	require.ErrorIs(t, err, ErrNoUsageData, "missing metrics must be reported as missing data")
}

func TestCheckRecordsFailureRatherThanLookingClean(t *testing.T) {
	t.Parallel()

	t.Run("a failed fetch", func(t *testing.T) {
		t.Parallel()

		result := check("pods", "desc", errors.New("connection refused"),
			func() ([]Issue, error) { return nil, nil })

		assert.False(t, result.Ran(), "the check did not run")
		assert.False(t, result.Clean(), "a failed check must never read as clean")
		assert.Contains(t, result.Err, "connection refused", "the reason is kept")
	})

	t.Run("a failed parse", func(t *testing.T) {
		t.Parallel()

		result := check("pods", "desc", nil,
			func() ([]Issue, error) { return nil, errors.New("bad json") })

		assert.False(t, result.Ran(), "the check did not run")
		assert.False(t, result.Clean(), "a failed check must never read as clean")
	})

	t.Run("a genuine clean result", func(t *testing.T) {
		t.Parallel()

		result := check("pods", "desc", nil, func() ([]Issue, error) { return nil, nil })

		assert.True(t, result.Ran(), "the check ran")
		assert.True(t, result.Clean(), "and found nothing")
	})
}
