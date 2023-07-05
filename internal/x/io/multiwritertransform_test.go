// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox_test

import (
	"bytes"
	"io"
	"testing"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

func setup(t *testing.T) (*bytes.Buffer, io.Writer) {
	t.Helper()

	stringBuffer := bytes.NewBufferString("")

	multiWriter := iox.MultiWriterTransform(iox.WriterTransform{
		W:          stringBuffer,
		Transforms: []bytesx.TransformFunc{bytesx.Identity},
	})

	return stringBuffer, multiWriter
}

func TestWrite(t *testing.T) {
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

			input := tc.input

			buf, multiWriter := setup(t)

			_, err := multiWriter.Write([]byte(input))
			if err != nil && !tc.wantErr {
				t.Fatalf("expected to not get an error: %v", err)
			}

			if err == nil && tc.wantErr {
				t.Fatalf("expected to get an error")
			}

			gotStr := buf.String()

			if gotStr != tc.wantStr {
				t.Errorf("want = %s, got = %s", tc.wantStr, gotStr)
			}
		})
	}
}
