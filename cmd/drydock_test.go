// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrydockCmdFlags(t *testing.T) {
	t.Parallel()

	c := NewDrydockCmd()
	assert.Equal(t, "drydock", c.Use)

	for name, want := range map[string]string{
		"address":         "127.0.0.1",
		"port":            "8080",
		"output":          "furyctl.yaml",
		"distro-location": "",
		"git-protocol":    "https",
		"no-browser":      "false",
	} {
		f := c.Flags().Lookup(name)
		require.NotNil(t, f, name)
		assert.Equal(t, want, f.DefValue, name)
	}
}

func TestDrydockIsRegistered(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()

	found := false

	for _, c := range root.Commands() {
		if c.Use == "drydock" {
			found = true
		}
	}

	assert.True(t, found)
}
