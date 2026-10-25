// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package onpremises //nolint:testpackage // calls the unexported function that gives the unsafe reducer lists.

import (
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The creator reads the reducer rules only. It takes its
// lists from the shared verdict with the unsafe reducer check alone: the immutable
// check would log an error line for each node condition it cannot evaluate, as it
// does in the preflight. The test is not parallel because it reads the global logger.
//
//nolint:paralleltest // the hook is on the global logger of logrus.
func TestCreator_UnsafeReducerCheckRunsAlone(t *testing.T) {
	t.Run("the unsafe reducer lists are computed without the immutable check", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)

		// The configuration of the extractor has no logging type and the changelog does
		// not change it: the node conditions of the immutable rules cannot be evaluated.
		first, second, err := unsafeReducerRules(charExtractor(t, characterizedRules), r3diff.Changelog{
			charChange(".spec.distribution.modules.monitoring.prometheus.retentionTime", nil, "15d"),
		})

		require.NoError(t, err)
		assert.Empty(t, first)
		assert.Empty(t, second)

		for _, entry := range hook.AllEntries() {
			assert.NotEqual(t, logrus.ErrorLevel, entry.Level, entry.Message)
		}
	})
}
