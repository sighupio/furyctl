// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/upgradeanalysis"
)

const schemaBefore = `{
  "type": "object",
  "$defs": {
    "Spec.Kubernetes.Advanced": {
      "type": "object",
      "properties": {
        "kubeProxy": {
          "type": "object",
          "properties": { "enabled": { "type": "boolean" } }
        }
      }
    },
    "Spec.Distribution.Modules.Logging.CustomOutputs": {
      "type": "object",
      "properties": { "audit": { "type": "string" } },
      "required": ["audit"]
    }
  }
}`

const schemaAfter = `{
  "type": "object",
  "$defs": {
    "Spec.Kubernetes.Advanced": {
      "type": "object",
      "properties": {
        "kubeProxy": {
          "type": "object",
          "properties": { "type": { "type": "string" } }
        }
      }
    },
    "Spec.Distribution.Modules.Logging.CustomOutputs": {
      "type": "object",
      "properties": { "audit": { "type": "string" }, "ingressHaproxy": { "type": "string" } },
      "required": ["audit", "ingressHaproxy"]
    }
  }
}`

// distDir writes a schema where a downloaded distribution would hold it.
func distDir(t *testing.T, schema string) string {
	t.Helper()

	dir := t.TempDir()
	public := filepath.Join(dir, "schemas", "public")
	require.NoError(t, os.MkdirAll(public, 0o755), "creating the schema directory")
	require.NoError(t,
		os.WriteFile(filepath.Join(public, "onpremises-kfd-v1alpha2.json"), []byte(schema), 0o600),
		"writing the schema",
	)

	return dir
}

// pathFetcher serves two versions, each with its own distribution directory.
type pathFetcher struct {
	from, to       string
	fromDir, toDir string
}

func (f pathFetcher) Fetch(_, version string) (upgradeanalysis.Distribution, error) {
	manifest := manifest{
		networking: "v3.1.0", ingress: "v5.0.1", monitoring: "v4.1.0", logging: "v5.3.0",
		tracing: "v1.4.0", opa: "v1.16.0", auth: "v0.6.1", dr: "v3.3.0",
		k8s: "1.34.4", installer: "v1.34.4",
	}.kfd()

	switch version {
	case f.from:
		return upgradeanalysis.Distribution{Manifest: manifest, Path: f.fromDir}, nil

	case f.to:
		return upgradeanalysis.Distribution{Manifest: config.KFD{}, Path: f.toDir}, nil

	default:
		return upgradeanalysis.Distribution{}, os.ErrNotExist
	}
}

func buildWithConfig(t *testing.T, cfg map[string]any) []upgradeanalysis.Finding {
	t.Helper()

	fsys := hopsFS([]string{"1.34.1-1.35.1"}, []string{"1.34.1-1.35.1"})

	fetcher := pathFetcher{
		from:    "v1.34.1",
		to:      "v1.35.1",
		fromDir: distDir(t, schemaBefore),
		toDir:   distDir(t, schemaAfter),
	}

	analysis, err := upgradeanalysis.Build(fsys, fetcher, qaCluster("v1.34.1"), cfg, "v1.35.1")
	require.NoError(t, err, "Build")
	require.Len(t, analysis.Hops, 1, "one hop")

	return analysis.Hops[0].Findings
}

// TestRemovedPropertyThatTheClusterSets is the kubeProxy.enabled case: the key is gone in
// v1.35.x, and schema validation does not catch it because the on-premises schema nests
// additionalProperties inside properties.
func TestRemovedPropertyThatTheClusterSets(t *testing.T) {
	t.Parallel()

	findings := buildWithConfig(t, map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"advanced": map[string]any{
					"kubeProxy": map[string]any{"enabled": true},
				},
			},
		},
	})

	require.Len(t, findings, 1, "exactly one finding")
	assert.Equal(t, upgradeanalysis.SeverityBlocker, findings[0].Severity, "severity")
	assert.Equal(t, "spec.kubernetes.advanced.kubeProxy.enabled", findings[0].Path, "path")
	assert.Contains(t, findings[0].Message, "no longer exists", "message")
	assert.Contains(t, findings[0].Message, "likely a rename", "the replacement must be hinted at")
	assert.Contains(t, findings[0].Message, "type", "the replacing property must be named")
}

func TestRemovedPropertyTheClusterDoesNotSet(t *testing.T) {
	t.Parallel()

	// The same schema change, but this cluster never set kubeProxy. Nothing to report.
	findings := buildWithConfig(t, map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{"advanced": map[string]any{}},
		},
	})

	assert.Empty(t, findings, "a property the cluster does not set is not a problem")
}

// TestNewlyRequiredProperty is the logging.customOutputs case, a breaking change the
// v1.34.0 release notes do not mention.
func TestNewlyRequiredProperty(t *testing.T) {
	t.Parallel()

	findings := buildWithConfig(t, map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"modules": map[string]any{
					"logging": map[string]any{
						"customOutputs": map[string]any{"audit": "..."},
					},
				},
			},
		},
	})

	require.Len(t, findings, 1, "exactly one finding")
	assert.Equal(t, upgradeanalysis.SeverityBlocker, findings[0].Severity, "severity")
	assert.Equal(t,
		"spec.distribution.modules.logging.customOutputs.ingressHaproxy",
		findings[0].Path, "path")
	assert.Contains(t, findings[0].Message, "becomes required", "message")
}

func TestNewlyRequiredPropertyWhenTheParentIsAbsent(t *testing.T) {
	t.Parallel()

	// A cluster on loki has no customOutputs block at all, so the new requirement cannot
	// apply to it and must not be reported.
	findings := buildWithConfig(t, map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"modules": map[string]any{
					"logging": map[string]any{"type": "loki"},
				},
			},
		},
	})

	assert.Empty(t, findings, "a requirement on an absent block does not apply")
}

func TestNewlyRequiredPropertyAlreadySet(t *testing.T) {
	t.Parallel()

	findings := buildWithConfig(t, map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"modules": map[string]any{
					"logging": map[string]any{
						"customOutputs": map[string]any{"audit": "...", "ingressHaproxy": "..."},
					},
				},
			},
		},
	})

	assert.Empty(t, findings, "a requirement the cluster already satisfies is not a finding")
}

func TestNoConfigMeansNoFindings(t *testing.T) {
	t.Parallel()

	// The report degrades to version deltas when the configuration cannot be read.
	assert.Empty(t, buildWithConfig(t, nil), "no configuration, no configuration findings")
}
