// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package upgradeanalysis_test

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

// analysisPackages are the packages that build the upgrade analysis. They are held to the
// rule below; the rest of furyctl legitimately names versions, for example in its
// compatibility matrix.
var analysisPackages = []string{".", "../upgradepath", "../schemadiff", "../clusterhealth"}

// versionLiteral matches a distribution or Kubernetes version such as 1.35 or v1.34.1.
// "v1alpha2" and byte sizes do not match: a digit-dot-digit pair is required.
var versionLiteral = regexp.MustCompile(`\bv?1\.[23][0-9]\b`)

// TestNoHardcodedVersions keeps every check in the upgrade analysis version-agnostic.
//
// A check may only use facts the tool can derive generically: live cluster state, furyctl's
// own shipped data, and the artifacts of the distribution versions involved — kfd.yaml,
// schemas/, rules/ and docs/releases/. Knowledge of what a particular release changed must
// never be written into furyctl's source: it is correct for exactly one version and
// misleading afterwards, and it makes the tool rot with every distribution release.
//
// Release-specific facts reach the reader from the release's own artifacts instead. The
// breaking changes of a version come from its docs/releases entry, and its unsupported
// configuration transitions from its rules/ file, both maintained per release by the
// distribution rather than here.
//
// This is a tripwire, not a proof. It catches a version written out in full, which is the
// usual shape of the mistake, but it cannot catch release knowledge expressed without one:
// a constant holding a bare minor, or a lookup table of operating system releases, both slip
// through. The principle itself is kept by review; this test only makes the common case
// impossible to merge by accident.
func TestNoHardcodedVersions(t *testing.T) {
	t.Parallel()

	for _, pkg := range analysisPackages {
		t.Run(filepath.Base(pkg), func(t *testing.T) {
			t.Parallel()

			err := filepath.WalkDir(pkg, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}

				source, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}

				for i, line := range strings.Split(string(source), "\n") {
					if match := versionLiteral.FindString(line); match != "" {
						assert.Fail(t,
							"a version literal in an upgrade-analysis package",
							"%s:%d names the version %q.\n\n"+
								"Checks must not encode what a particular release changed: that is correct "+
								"for one version and misleading after it.\n"+
								"Derive the fact instead, or take it from the release's own artifacts:\n"+
								"  docs/releases/<version>.md for breaking changes\n"+
								"  rules/<kind>-kfd-v1alpha2.yaml for unsupported transitions\n"+
								"\n  %s",
							path, i+1, match, strings.TrimSpace(line))
					}
				}

				return nil
			})

			require.NoError(t, err, "walking %s", pkg)
		})
	}
}
