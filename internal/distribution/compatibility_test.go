// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package distribution_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/distribution"
)

func TestEKSClusterCheckIsCompatible(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		distributionVersion string
		expected            bool
	}{
		{
			name:                "should return true if distribution version is greater than or equals 1.25.6 and less than or equals 1.25.10",
			distributionVersion: "v1.25.10",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is less than 1.25.6",
			distributionVersion: "v1.25.5",
			expected:            false,
		},
		{
			name:                "should return false if distribution version is invalid",
			distributionVersion: "invalid",
			expected:            false,
		},
		{
			name:                "should return false if distribution version is greater than 1.25.10",
			distributionVersion: "v1.25.11",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.26.0 and less than or equals 1.26.6",
			distributionVersion: "v1.26.4",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.26.6",
			distributionVersion: "v1.26.7",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.27.0 and less than or equals 1.27.8",
			distributionVersion: "v1.27.2",
			expected:            true,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.27.0 and less than or equals 1.27.8",
			distributionVersion: "v1.27.8",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.27.9",
			distributionVersion: "v1.27.10",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.28.0 and less than or equals 1.28.5",
			distributionVersion: "v1.28.3",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.28.6",
			distributionVersion: "v1.28.7",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.29.0 and less than or equals 1.29.7",
			distributionVersion: "v1.29.7",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.29.7",
			distributionVersion: "v1.29.8",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than 1.30.0 and less than 1.30.2",
			distributionVersion: "v1.30.2",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.30.2",
			distributionVersion: "v1.30.3",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than 1.31.0 and less or equal than 1.31.1",
			distributionVersion: "v1.31.0",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.31.1",
			distributionVersion: "v1.31.2",
			expected:            false,
		},
		{
			name:                "should return true if distribution version equals 1.32.0",
			distributionVersion: "v1.32.0",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.32.0",
			distributionVersion: "v1.32.2",
			expected:            false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := distribution.NewEKSClusterCheck(tc.distributionVersion)

			got := checker.IsCompatible()

			if got != tc.expected {
				t.Errorf("IsCompatible() got = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestKFDDistributionCheckIsCompatible(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		distributionVersion string
		expected            bool
	}{
		{
			name:                "should return true if distribution version is greater than or equals 1.25.6 and less than or equals 1.25.10",
			distributionVersion: "v1.25.10",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is less than 1.25.6",
			distributionVersion: "v1.25.5",
			expected:            false,
		},
		{
			name:                "should return false if distribution version is invalid",
			distributionVersion: "invalid",
			expected:            false,
		},
		{
			name:                "should return false if distribution version is greater than 1.25.10",
			distributionVersion: "v1.25.11",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.26.0 and less than or equals 1.26.6",
			distributionVersion: "v1.26.4",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.26.6",
			distributionVersion: "v1.26.7",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.27.0 and less than or equals 1.27.8",
			distributionVersion: "v1.27.2",
			expected:            true,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.27.0 and less than or equals 1.27.8",
			distributionVersion: "v1.27.8",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.27.9",
			distributionVersion: "v1.27.10",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.28.0 and less than or equals 1.28.5",
			distributionVersion: "v1.28.3",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.28.6",
			distributionVersion: "v1.28.7",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.29.0 and less than or equals 1.29.7",
			distributionVersion: "v1.29.3",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.29.7",
			distributionVersion: "v1.29.8",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.30.0 or less than or equals 1.30.2",
			distributionVersion: "v1.30.2",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.30.2",
			distributionVersion: "v1.30.3",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.31.0 or less than or equals 1.31.1",
			distributionVersion: "v1.31.1",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.31.1",
			distributionVersion: "v1.31.2",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.32.0",
			distributionVersion: "v1.32.0",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.32.0",
			distributionVersion: "v1.32.1",
			expected:            false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := distribution.NewKFDDistributionCheck(tc.distributionVersion)

			got := checker.IsCompatible()

			if got != tc.expected {
				t.Errorf("IsCompatible() got = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestOnPremisesCheckIsCompatible(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		distributionVersion string
		expected            bool
	}{
		{
			name:                "should return true if distribution version is greater than 1.25.8 and less than 1.25.11",
			distributionVersion: "v1.25.9",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is less than 1.25.8",
			distributionVersion: "v1.25.5",
			expected:            false,
		},
		{
			name:                "should return false if distribution version is invalid",
			distributionVersion: "invalid",
			expected:            false,
		},
		{
			name:                "should return false if distribution version is greater than 1.25.10",
			distributionVersion: "v1.25.11",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than 1.26.2 and less than 1.26.6",
			distributionVersion: "v1.26.4",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.26.6",
			distributionVersion: "v1.26.7",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than 1.27.0 and less than 1.27.10",
			distributionVersion: "v1.27.2",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.27.9",
			distributionVersion: "v1.27.10",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than 1.28.0 and less than 1.28.5",
			distributionVersion: "v1.28.3",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.28.6",
			distributionVersion: "v1.28.7",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than 1.29.1 and less than 1.29.7",
			distributionVersion: "v1.29.3",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.29.7",
			distributionVersion: "v1.29.8",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.30.0 or less than or equals 1.30.2",
			distributionVersion: "v1.30.2",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.30.2",
			distributionVersion: "v1.30.3",
			expected:            false,
		},
		{
			name:                "should return true if distribution version is greater than or equals 1.31.0 and less than or equals 1.31.1",
			distributionVersion: "v1.31.1",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.31.1",
			distributionVersion: "v1.31.2",
			expected:            false,
		},
		{
			name:                "should return true if distribution version equals 1.32.0",
			distributionVersion: "v1.32.0",
			expected:            true,
		},
		{
			name:                "should return false if distribution version is greater than 1.32.0",
			distributionVersion: "v1.32.1",
			expected:            false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := distribution.NewOnPremisesCheck(tc.distributionVersion)

			got := checker.IsCompatible()

			if got != tc.expected {
				t.Errorf("IsCompatible() got = %v, want %v", got, tc.expected)
			}
		})
	}
}
