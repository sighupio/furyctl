// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytesx_test

import (
	"bytes"
	"testing"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
)

func TestSafeWriteToBuffer(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		setup   func() ([]any, string)
		wantStr string
		wantErr bool
	}{
		{
			"empty string value",
			func() ([]any, string) {
				var values []any

				values = append(values, "")

				return values, "test: %v"
			},
			"test: ",
			false,
		},
		{
			"empty value",
			func() ([]any, string) {
				var values []any

				values = append(values, nil)

				return values, "test"
			},
			"test",
			false,
		},
		{
			"empty values",
			func() ([]any, string) {
				var values []any

				return values, "test"
			},
			"test",
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			var err error

			buffer := bytes.NewBufferString("")

			values, content := tc.setup()

			err = bytesx.SafeWriteToBuffer(buffer, content, values...)

			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantStr != buffer.String() {
				t.Fatalf("want %s got %v", tc.wantStr, buffer.String())
			}
		})
	}
}
