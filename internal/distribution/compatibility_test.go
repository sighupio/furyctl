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
		wantErr             bool
	}{
		{
			name:                "should return true if distribution version is greater than 1.25.6 and less than 1.25.11",
			distributionVersion: "v1.25.7",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is less than 1.25.6",
			distributionVersion: "v1.25.5",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is invalid",
			distributionVersion: "invalid",
			expected:            false,
			wantErr:             true,
		},
		{
			name:                "should return true if distribution version is greater than 1.26.0 and less than 1.26.6",
			distributionVersion: "v1.26.4",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.25.10",
			distributionVersion: "v1.25.11",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.26.5",
			distributionVersion: "v1.26.6",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.27.3",
			distributionVersion: "v1.27.4",
			expected:            false,
			wantErr:             false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := distribution.NewEKSClusterCheck(tc.distributionVersion)

			got, err := checker.IsCompatible()
			if (err != nil) != tc.wantErr {
				t.Errorf("IsCompatible() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

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
		wantErr             bool
	}{
		{
			name:                "should return true if distribution version is greater than 1.25.6 and less than 1.25.10",
			distributionVersion: "v1.25.9",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is less than 1.25.6",
			distributionVersion: "v1.25.5",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is invalid",
			distributionVersion: "invalid",
			expected:            false,
			wantErr:             true,
		},
		{
			name:                "should return false if distribution version is greater than 1.25.10",
			distributionVersion: "v1.25.11",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return true if distribution version is greater than 1.26.0 and less than 1.26.6",
			distributionVersion: "v1.26.4",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.26.5",
			distributionVersion: "v1.26.6",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return true if distribution version is greater than 1.27.0 and less than 1.27.4",
			distributionVersion: "v1.27.2",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.27.3",
			distributionVersion: "v1.27.4",
			expected:            false,
			wantErr:             false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := distribution.NewKFDDistributionCheck(tc.distributionVersion)

			got, err := checker.IsCompatible()
			if (err != nil) != tc.wantErr {
				t.Errorf("IsCompatible() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

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
		wantErr             bool
	}{
		{
			name:                "should return true if distribution version is greater than 1.25.8 and less than 1.25.11",
			distributionVersion: "v1.25.9",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is less than 1.25.8",
			distributionVersion: "v1.25.5",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is invalid",
			distributionVersion: "invalid",
			expected:            false,
			wantErr:             true,
		},
		{
			name:                "should return false if distribution version is greater than 1.25.10",
			distributionVersion: "v1.25.11",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return true if distribution version is greater than 1.26.2 and less than 1.26.6",
			distributionVersion: "v1.26.4",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.26.5",
			distributionVersion: "v1.26.6",
			expected:            false,
			wantErr:             false,
		},
		{
			name:                "should return true if distribution version is greater than 1.27.0 and less than 1.27.4",
			distributionVersion: "v1.27.2",
			expected:            true,
			wantErr:             false,
		},
		{
			name:                "should return false if distribution version is greater than 1.27.3",
			distributionVersion: "v1.27.4",
			expected:            false,
			wantErr:             false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := distribution.NewOnPremisesCheck(tc.distributionVersion)

			got, err := checker.IsCompatible()
			if (err != nil) != tc.wantErr {
				t.Errorf("IsCompatible() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if got != tc.expected {
				t.Errorf("IsCompatible() got = %v, want %v", got, tc.expected)
			}
		})
	}
}
