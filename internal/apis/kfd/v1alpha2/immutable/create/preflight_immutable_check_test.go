// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // builds a PreFlight with only its paths, which are unexported.

import (
	"os"
	"path/filepath"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/pkg/diffs"
)

// reducerRuleWithNodeCondition is a rule file with one reducer rule whose safe
// condition reads a node that is neither in the changes nor in the configuration:
// judging that rule logs an error line.
const reducerRuleWithNodeCondition = `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "the logging type changed"
    safe:
      - fromNodes:
          - path: ".spec.distribution.modules.monitoring.type"
            from: "none"
    reducers:
      - key: distributionModulesLoggingType
        lifecycle: pre-apply
`

// CheckStateDiffs reads the immutable rules only. It takes
// its decision from the shared verdict with the immutable check alone, so the reducer
// rules are not judged there. The test is not parallel because it reads the global
// logger.
//
//nolint:paralleltest // the hook is on the global logger of logrus.
func TestPreFlight_ImmutableCheckRunsAlone(t *testing.T) {
	t.Run("the immutable check does not judge the reducer rules", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)

		distroPath := t.TempDir()

		require.NoError(t, os.Mkdir(filepath.Join(distroPath, "rules"), 0o750))
		require.NoError(t, os.WriteFile(
			filepath.Join(distroPath, "rules", "immutable-kfd-v1alpha2.yaml"),
			[]byte(reducerRuleWithNodeCondition),
			0o600,
		))

		checker := diffs.NewBaseChecker(map[string]any{}, map[string]any{})
		preflight := &PreFlight{paths: cluster.CreatorPaths{DistroPath: distroPath}}
		changelog := r3diff.Changelog{
			charChange(".spec.distribution.modules.logging.type", "loki", "opensearch"),
		}

		require.NoError(t, preflight.CheckStateDiffs(changelog, checker))

		for _, entry := range hook.AllEntries() {
			assert.NotEqual(t, logrus.ErrorLevel, entry.Level, entry.Message)
		}
	})
}
