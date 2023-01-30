// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytesx_test

import (
	"strings"
	"testing"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
)

func TestStripColor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		input   string
		wantStr string
		wantErr bool
	}{
		{
			"empty string",
			"",
			"",
			false,
		},
		{
			"no color",
			"test",
			"test",
			false,
		},
		{
			"color",
			"\x1b[31mtest\x1b[0m",
			"test",
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			input := []byte(tc.input)

			gotStr, err := bytesx.StripColor(input)
			if err != nil && !tc.wantErr {
				t.Fatalf("expected to not get an error: %v", err)
			}

			if err == nil && tc.wantErr {
				t.Fatalf("expected to get an error")
			}

			if string(gotStr) != tc.wantStr {
				t.Errorf("want = %s, got = %s", tc.wantStr, gotStr)
			}
		})
	}
}

func TestToJSONLogFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		setup   func() (string, string, *string)
		wantStr string
		wantErr bool
	}{
		{
			"empty string",
			func() (string, string, *string) {
				action := "test"

				return "", "debug", &action
			},
			"\"level\":\"debug\",\"action\":\"test\",\"msg\":\"\"",
			false,
		},
		{
			"nil action",
			func() (string, string, *string) {
				return "", "debug", nil
			},
			"\"level\":\"debug\",\"msg\":\"\"",
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			input, level, action := tc.setup()

			gotStr, err := bytesx.ToJSONLogFormat(level, action)([]byte(input))
			if err != nil && !tc.wantErr {
				t.Fatalf("expected to not get an error: %v", err)
			}

			if err == nil && tc.wantErr {
				t.Fatalf("expected to get an error")
			}

			if !strings.Contains(string(gotStr), tc.wantStr) {
				t.Errorf("want = %s, got = %s", tc.wantStr, gotStr)
			}
		})
	}
}

func TestIdentity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		input   string
		wantStr string
		wantErr bool
	}{
		{
			"empty string",
			"",
			"",
			false,
		},
		{
			"simple string",
			"test",
			"test",
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			input := []byte(tc.input)

			gotStr, err := bytesx.Identity(input)
			if err != nil && !tc.wantErr {
				t.Fatalf("expected to not get an error: %v", err)
			}

			if err == nil && tc.wantErr {
				t.Fatalf("expected to get an error")
			}

			if string(gotStr) != tc.wantStr {
				t.Errorf("want = %s, got = %s", tc.wantStr, gotStr)
			}
		})
	}
}
