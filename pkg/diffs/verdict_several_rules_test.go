// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
)

// These tests pin which rule gives the text and the verdict when a change matches
// more than one, and that a leaf rule reached by expansion marks its own leaf only.

func TestVerdicts_SeveralRulesOnOneChange(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		rules           string
		change          field
		wantVerdict     diffs.Verdict
		wantText        string
		wantImmutable   bool
		wantUnsupported bool
		wantUnsafe      int
	}{
		{
			name: "the first of two immutable rules gives the text",
			rules: `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: true
    description: "first immutable rule"
  - path: .spec.distribution.modules.logging.type
    immutable: true
    description: "second immutable rule"
`,
			change:        field{pathLoggingType, "loki", "opensearch"},
			wantVerdict:   diffs.VerdictBlocked,
			wantText:      "first immutable rule",
			wantImmutable: true,
		},
		{
			name: "the first of two unsupported rules gives the reason",
			rules: `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: false
    unsupported:
      - from: loki
        to: opensearch
        reason: "first reason"
  - path: .spec.distribution.modules.logging.type
    immutable: false
    unsupported:
      - from: loki
        to: opensearch
        reason: "second reason"
`,
			change:          field{pathLoggingType, "loki", "opensearch"},
			wantVerdict:     diffs.VerdictBlocked,
			wantText:        "first reason",
			wantUnsupported: true,
		},
		{
			name: "the first of two unsafe rules gives the text",
			rules: `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "first unsafe rule"
    reducers:
      - key: firstLoggingType
        lifecycle: pre-apply
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "second unsafe rule"
    reducers:
      - key: secondLoggingType
        lifecycle: pre-apply
`,
			change:      field{pathLoggingType, "loki", "opensearch"},
			wantVerdict: diffs.VerdictNeedsConfirmation,
			wantText:    "first unsafe rule",
			wantUnsafe:  2,
		},
		{
			name: "the first of two safe rules gives the text",
			rules: `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "first safe rule"
    safe:
      - from: none
    reducers:
      - key: firstLoggingType
        lifecycle: pre-apply
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "second safe rule"
    safe:
      - from: none
    reducers:
      - key: secondLoggingType
        lifecycle: pre-apply
`,
			change:      field{pathLoggingType, "none", "loki"},
			wantVerdict: diffs.VerdictMigration,
			wantText:    "first safe rule",
		},
		{
			name: "needs confirmation wins over migration",
			rules: `distribution:
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "safe rule"
    safe:
      - from: none
    reducers:
      - key: firstLoggingType
        lifecycle: pre-apply
  - path: .spec.distribution.modules.logging.type
    immutable: false
    description: "unsafe rule"
    reducers:
      - key: secondLoggingType
        lifecycle: pre-apply
`,
			change:      field{pathLoggingType, "none", "loki"},
			wantVerdict: diffs.VerdictNeedsConfirmation,
			wantText:    "unsafe rule",
			wantUnsafe:  1,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			verdicts := verdictsFor(t, kindImmutable, syntheticRules(t, tC.rules), tC.change)

			require.Len(t, verdicts, 1)

			got := verdictAt(t, verdicts, tC.change.path)

			assert.Equal(t, tC.wantVerdict, got.Verdict)
			assert.Equal(t, tC.wantText, got.Text)
			assert.Equal(t, tC.wantImmutable, got.Immutable, "immutable violation")
			assert.Equal(t, tC.wantUnsupported, got.Unsupported, "unsupported violation")
			assert.Len(t, got.UnsafeReducers[phaseDistribution], tC.wantUnsafe)
		})
	}
}

func TestVerdicts_SiblingLeafOfAnAddedObject(t *testing.T) {
	t.Parallel()

	t.Run("a leaf rule reached by expansion leaves the other leaves of the object alone",
		func(t *testing.T) {
			t.Parallel()

			verdicts := verdictsFor(t, kindOnPremises, realRules, field{
				pathKubeProxy, nil, map[string]any{"type": "none", "unruled": "value"},
			})

			require.Len(t, verdicts, 2)

			ruled := verdictAt(t, verdicts, pathKubeProxyType)
			assert.Equal(t, diffs.VerdictBlocked, ruled.Verdict)
			assert.True(t, ruled.Unsupported)

			sibling := verdictAt(t, verdicts, pathKubeProxy+".unruled")
			assert.Equal(t, diffs.VerdictNone, sibling.Verdict)
			assert.False(t, sibling.Unsupported)
			assert.False(t, sibling.Immutable)
			assert.Empty(t, sibling.Text)
			assert.Empty(t, sibling.UnsafeReducers[phaseKubernetes])
		})
}
