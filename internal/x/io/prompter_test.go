// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox_test

import (
	"bufio"
	"strings"
	"testing"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

func TestPrompter_Ask(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		input      string
		promptWord string
		expected   bool
	}{
		{
			name:       "user writes correct prompt input",
			input:      "yes\n",
			promptWord: "yes",
			expected:   true,
		},
		{
			name:       "user writes correct prompt input with multiple endline",
			input:      "yes\n\n\n",
			promptWord: "yes",
			expected:   true,
		},
		{
			name:       "user writes wrong prompt input",
			input:      "yessh\n",
			promptWord: "yes",
			expected:   false,
		},
		{
			name:       "user writes correct uppercase prompt input",
			input:      "YES\n\n\n",
			promptWord: "yes",
			expected:   true,
		},
		{
			name:       "user writes correct prompt input with whitespace",
			input:      "   yes   \n\n\n",
			promptWord: "yes",
			expected:   true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prompter := iox.NewPrompter(bufio.NewReader(strings.NewReader(tc.input)))

			prompt, err := prompter.Ask(tc.promptWord)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if prompt != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, !tc.expected)
			}
		})
	}
}
