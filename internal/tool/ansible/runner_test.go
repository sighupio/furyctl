// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package ansible_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Version(t *testing.T) {
	r := ansible.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), ansible.Paths{
		Ansible:         "ansible",
		AnsiblePlaybook: "ansible-playbook",
		WorkDir:         os.TempDir(),
	})

	got, err := r.Version()
	require.NoError(t, err)

	assert.Equal(t, "v1.2.3", got)
}

func Test_PathsForVersion(t *testing.T) {
	t.Parallel()

	// Backward compat: no pinned version -> host ansible (bare command names, no python/collections).
	host := ansible.PathsForVersion("/bin", "", "/work")
	assert.Equal(t, "ansible", host.Ansible)
	assert.Equal(t, "ansible-playbook", host.AnsiblePlaybook)
	assert.Empty(t, host.Python)
	assert.Empty(t, host.CollectionsPath)

	// Pinned version -> mise-managed layout under <bin>/ansible/<ver>/.
	managed := ansible.PathsForVersion("/bin", "2.21.0", "/work")
	for name, got := range map[string]string{
		"Ansible":         managed.Ansible,
		"AnsiblePlaybook": managed.AnsiblePlaybook,
		"Python":          managed.Python,
		"CollectionsPath": managed.CollectionsPath,
	} {
		assert.NotEmpty(t, got, "%s should not be empty", name)
		assert.True(t, strings.HasPrefix(got, "/bin/ansible/2.21.0/"), "%s = %q, want under /bin/ansible/2.21.0/", name, got)
	}
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "ansible":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "v1.2.3")
		default:
			fmt.Fprintf(os.Stdout, "subcommand '%s' not found", subcmd)
		}
	default:
		fmt.Fprintf(os.Stdout, "command '%s' not found", cmd)
	}

	os.Exit(0)
}
