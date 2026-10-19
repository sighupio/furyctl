// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package terraform

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONMessageWriter(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	w := &jsonMessageWriter{w: &out}

	// Split mid-line to check buffering across writes.
	chunks := []string{
		`{"@level":"info","@message":"aws_vpc.main: Creating..."}` + "\n" + `{"@mess`,
		`age":"Apply complete!"}` + "\n",
		"plain text\n",
		`{"type":"version"}` + "\n",
		"no newline yet",
	}

	for _, c := range chunks {
		n, err := w.Write([]byte(c))
		require.NoError(t, err)
		assert.Equal(t, len(c), n)
	}

	assert.Equal(t,
		"aws_vpc.main: Creating...\nApply complete!\nplain text\n"+`{"type":"version"}`+"\n",
		out.String(),
	)
}

func TestJSONText(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"@level":"info","@message":"aws_vpc.main: Creating..."}`,
		"plain text",
		`{"@level":"error","@message":"Error: creating VPC","type":"diagnostic",` +
			`"diagnostic":{"severity":"error","summary":"creating VPC","detail":"limit reached\nask for more"}}`,
		`{"@level":"warn","@message":"Warning: old","diagnostic":{"detail":"not shown"}}`,
	}

	assert.Equal(t, []string{
		"aws_vpc.main: Creating...",
		"plain text",
		"Error: creating VPC",
		"  limit reached",
		"  ask for more",
		"Warning: old",
	}, jsonText(lines))
}
