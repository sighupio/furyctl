// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffs_test

import (
	"fmt"
	"testing"

	diffx "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
)

func TestBaseChecker_GenerateDiff(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc          string
		currentCfg    map[string]any
		newCfg        map[string]any
		expectedDiffs diffx.Changelog
		wantErr       bool
		wantErrMsg    string
	}{
		{
			desc: "no diffs",
			currentCfg: map[string]any{
				"foo": "bar",
			},
			newCfg: map[string]any{
				"foo": "bar",
			},
			expectedDiffs: diffx.Changelog{},
			wantErr:       false,
		},
		{
			desc: "diffs found - simple",
			currentCfg: map[string]any{
				"foo": "bar",
			},
			newCfg: map[string]any{
				"foo": "baz",
			},
			expectedDiffs: diffx.Changelog{
				{
					Type: diffx.UPDATE,
					Path: []string{"foo"},
					From: "bar",
					To:   "baz",
				},
			},
			wantErr: false,
		},
		{
			desc:       "diffs found - current config nil",
			currentCfg: nil,
			newCfg: map[string]any{
				"foo": "baz",
			},
			expectedDiffs: diffx.Changelog{
				{
					Type: diffx.CREATE,
					Path: []string{"foo"},
					To:   "baz",
				},
			},
			wantErr: false,
		},
		{
			desc: "diffs found - new config nil",
			currentCfg: map[string]any{
				"foo": "bar",
			},
			newCfg: nil,
			expectedDiffs: diffx.Changelog{
				{
					Type: diffx.DELETE,
					Path: []string{"foo"},
					From: "bar",
				},
			},
			wantErr: false,
		},
		{
			desc:          "no diffs - nil",
			currentCfg:    nil,
			newCfg:        nil,
			expectedDiffs: diffx.Changelog{},
			wantErr:       false,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(tC.currentCfg, tC.newCfg)

			diffs, err := checker.GenerateDiff()
			if tC.wantErr {
				require.Error(t, err)
				assert.Equal(t, tC.wantErrMsg, err.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tC.expectedDiffs, diffs)
		})
	}
}

func TestBaseChecker_AssertImmutableViolations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc           string
		diffs          diffx.Changelog
		immutablePaths []string
		expectedErrs   []error
	}{
		{
			desc:           "no diffs",
			diffs:          diffx.Changelog{},
			immutablePaths: []string{},
			expectedErrs:   []error{},
		},
		{
			desc: "no immutable paths",
			diffs: diffx.Changelog{
				{
					Type: diffx.CREATE,
					Path: []string{"foo"},
					To:   1,
				},
			},
			immutablePaths: []string{},
			expectedErrs:   []error{},
		},
		{
			desc: "immutable paths found",
			diffs: diffx.Changelog{
				{
					Type: diffx.UPDATE,
					Path: []string{"spec", "foo", "2", "bar"},
					From: "bar",
					To:   "baz",
				},
				{
					Type: diffx.UPDATE,
					Path: []string{"spec", "bar", "baz"},
					From: "baz",
					To:   "bar",
				},
			},
			immutablePaths: []string{".spec.foo.*.bar", ".spec.bar.baz", ".spec.test.key"},
			expectedErrs: []error{
				fmt.Errorf("immutable value changed: path .spec.foo.2.bar  oldValue bar newValue baz"),
				fmt.Errorf("immutable value changed: path .spec.bar.baz  oldValue baz newValue bar"),
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			checker := diffs.NewBaseChecker(nil, nil)

			errs := checker.AssertImmutableViolations(tC.diffs, tC.immutablePaths)

			assert.Len(t, errs, len(tC.expectedErrs))

			for i, err := range errs {
				assert.Equal(t, tC.expectedErrs[i].Error(), err.Error())
			}
		})
	}
}
