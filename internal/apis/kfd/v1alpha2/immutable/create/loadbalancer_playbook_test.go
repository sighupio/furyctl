// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // exercises the unexported playbook check.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A distribution released before the load balancer upgrade gives no playbook. An upgrade
// between two of those versions must skip that work, and not stop.
func TestLoadBalancerPlaybookShipped(t *testing.T) {
	t.Parallel()

	// templatesDir builds the folder of the infrastructure ansible templates of a
	// distribution, with the files that it gives.
	templatesDir := func(t *testing.T, names ...string) string {
		t.Helper()

		dir := t.TempDir()
		for _, name := range names {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("---\n"), 0o600))
		}

		return dir
	}

	t.Run("a distribution that gives the playbook", func(t *testing.T) {
		t.Parallel()

		dir := templatesDir(t, "apply.yaml", "hosts.yaml.tpl", "upgrade-load-balancers.yml.tpl")

		shipped, err := loadBalancerPlaybookShipped(dir)
		require.NoError(t, err)
		assert.True(t, shipped)
	})

	t.Run("a distribution that gives it without the rendered suffix", func(t *testing.T) {
		t.Parallel()

		dir := templatesDir(t, "apply.yaml", "upgrade-load-balancers.yml")

		shipped, err := loadBalancerPlaybookShipped(dir)
		require.NoError(t, err)
		assert.True(t, shipped)
	})

	t.Run("a distribution released before the feature", func(t *testing.T) {
		t.Parallel()

		dir := templatesDir(t, "apply.yaml", "hosts.yaml.tpl", "ansible.cfg.tpl")

		shipped, err := loadBalancerPlaybookShipped(dir)
		require.NoError(t, err)
		assert.False(t, shipped)
	})

	// The reason the test reads the distribution and not the phase folder: a render
	// writes the files of the distribution over that folder, but it removes nothing.
	// A folder that an earlier run left with a playbook must not make an older
	// distribution read as one that gives it.
	t.Run("a phase folder that an earlier run left with a playbook", func(t *testing.T) {
		t.Parallel()

		phasePath := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(phasePath, "ansible"), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(phasePath, "ansible", "upgrade-load-balancers.yml"), []byte("---\n"), 0o600))

		// The distribution of this run gives no playbook.
		dir := templatesDir(t, "apply.yaml", "hosts.yaml.tpl")

		shipped, err := loadBalancerPlaybookShipped(dir)
		require.NoError(t, err)
		assert.False(t, shipped, "the playbook of an earlier run must not count")
	})

	t.Run("a distribution folder that is absent", func(t *testing.T) {
		t.Parallel()

		shipped, err := loadBalancerPlaybookShipped(filepath.Join(t.TempDir(), "absent"))
		require.NoError(t, err)
		assert.False(t, shipped)
	})
}
