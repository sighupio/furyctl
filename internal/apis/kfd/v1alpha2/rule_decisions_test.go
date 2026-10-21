// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package v1alpha2_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// ownRuleDecision matches a call to one of the three functions a kind uses to take
	// a rule decision by itself.
	ownRuleDecision = regexp.MustCompile(
		`\b(AssertImmutableViolations|AssertReducerUnsupportedViolations|UnsafeReducerRulesByDiffs)\(`,
	)

	// sharedVerdict matches a call to the shared rule verdict of pkg/diffs.
	sharedVerdict = regexp.MustCompile(`\bdiffs\.(DecideRules|Verdicts)\(`)

	// kindProfile matches any lookup of the profile of a kind.
	kindProfile = regexp.MustCompile(`\bdiffs\.(Require)?ProfileForKind\(`)

	// optionalKindProfile matches the lookup that reports a kind without a profile with a
	// boolean, which a call site can ignore.
	optionalKindProfile = regexp.MustCompile(`\bdiffs\.ProfileForKind\(`)
)

// requiredKindProfile matches the lookup of the profile of a kind by the constant of
// that kind, which fails on a kind without a profile. The kind is the folder of its
// apply code.
func requiredKindProfile(kind string) *regexp.Regexp {
	constants := map[string]string{"onpremises": "OnPremisesKind", "immutable": "ImmutableKind"}

	return regexp.MustCompile(`\bdiffs\.RequireProfileForKind\(distribution\.` + constants[kind] + `\)`)
}

// nonTestSources reads the Go files of a kind that are not tests, by path.
func nonTestSources(t *testing.T, kind string) map[string]string {
	t.Helper()

	sources := map[string]string{}

	err := filepath.WalkDir(kind, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		sources[filepath.ToSlash(path)] = string(content)

		return nil
	})

	require.NoError(t, err)
	require.NotEmpty(t, sources, "no Go file under %s", kind)

	return sources
}

func filesMatching(sources map[string]string, pattern *regexp.Regexp) []string {
	files := []string{}

	for path, content := range sources {
		if pattern.MatchString(content) {
			files = append(files, path)
		}
	}

	return files
}

func TestRuleDecisions_ComeFromTheSharedVerdict(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"onpremises", "immutable"} {
		t.Run(kind+" has no rule decision of its own", func(t *testing.T) {
			t.Parallel()

			sources := nonTestSources(t, kind)

			assert.Empty(t, filesMatching(sources, ownRuleDecision))

			for _, applyFile := range []string{kind + "/create/preflight.go", kind + "/creator.go"} {
				require.Contains(t, sources, applyFile)

				// A boolean keeps the source of the file out of the failure message.
				assert.True(t, sharedVerdict.MatchString(sources[applyFile]), "%s does not call the shared verdict", applyFile)
				assert.True(
					t,
					requiredKindProfile(kind).MatchString(sources[applyFile]),
					"%s does not read the profile of its kind by the kind constant",
					applyFile,
				)
			}

			// A lookup that only reports a missing profile could be ignored, and the
			// zero profile runs no rule check.
			assert.Empty(t, filesMatching(sources, optionalKindProfile))
		})
	}
}

func TestRuleDecisions_ProfileLookupFailsOnAKindWithoutProfile(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"onpremises", "immutable"} {
		t.Run(kind+" stops on a kind without a rule profile", func(t *testing.T) {
			t.Parallel()

			sources := nonTestSources(t, kind)

			for _, applyFile := range []string{kind + "/create/preflight.go", kind + "/creator.go"} {
				calls := requiredKindProfile(kind).FindAllStringIndex(sources[applyFile], -1)
				require.NotEmpty(t, calls, applyFile)

				// Every lookup is followed by the check of its error.
				for _, call := range calls {
					next := sources[applyFile][call[1]:]
					assert.True(
						t,
						strings.HasPrefix(next, "\n\tif err != nil {\n"),
						"%s ignores the error of the profile lookup",
						applyFile,
					)
				}
			}
		})
	}
}

func TestRuleDecisions_StayInTheOtherTwoKinds(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"ekscluster", "kfddistribution"} {
		t.Run(kind+" keeps its own rule decisions", func(t *testing.T) {
			t.Parallel()

			sources := nonTestSources(t, kind)

			// The apply code of these kinds is not part of the change: it still takes
			// its rule decisions by itself, in the two files that did it before.
			assert.ElementsMatch(
				t,
				[]string{kind + "/create/preflight.go", kind + "/creator.go"},
				filesMatching(sources, ownRuleDecision),
			)
			assert.Empty(t, filesMatching(sources, sharedVerdict))
			assert.Empty(t, filesMatching(sources, kindProfile))
		})
	}
}
