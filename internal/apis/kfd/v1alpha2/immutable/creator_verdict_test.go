// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable //nolint:testpackage // calls the unexported function that gives the unsafe reducer lists.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The creator of Immutable takes its unsafe reducer lists from the shared verdict. For
// every characterized changelog the lists are the characterized ones, and the ones
// the extractor calls of the base creator give.
func TestCreator_UnsafeReducersFromTheVerdict(t *testing.T) {
	t.Parallel()

	for _, tC := range charUnsafeReducersCases() {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			distroPath := characterizedRules
			if tC.noRules {
				distroPath = t.TempDir()
			}

			infrastructure, distribution, err := unsafeReducerRules(charExtractor(t, distroPath), tC.changelog)
			require.NoError(t, err)

			assert.Equal(t, tC.wantInfrastructure, charRules(infrastructure), "infrastructure")
			assert.Equal(t, tC.wantDistribution, charRules(distribution), "distribution")

			baseInfrastructure, baseDistribution := charBaseUnsafeReducers(charExtractor(t, distroPath), tC.changelog)

			assert.Equal(t, charRules(baseInfrastructure), charRules(infrastructure), "infrastructure of the base creator")
			assert.Equal(t, charRules(baseDistribution), charRules(distribution), "distribution of the base creator")
		})
	}
}
