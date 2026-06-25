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
	if err != nil {
		t.Fatal(err)
	}

	want := "v1.2.3"

	if got != want {
		t.Errorf("expected version '%s', got '%s'", want, got)
	}
}

func Test_PathsForVersion(t *testing.T) {
	t.Parallel()

	// Backward compat: no pinned version -> host ansible (bare command names, no python/collections).
	host := ansible.PathsForVersion("/bin", "", "/work")
	if host.Ansible != "ansible" || host.AnsiblePlaybook != "ansible-playbook" {
		t.Errorf("host fallback should use bare names, got %+v", host)
	}

	if host.Python != "" || host.CollectionsPath != "" {
		t.Errorf("host fallback must not set python/collections, got %+v", host)
	}

	// Pinned version -> mise-managed layout under <bin>/ansible/<ver>/.
	managed := ansible.PathsForVersion("/bin", "2.21.0", "/work")
	for name, got := range map[string]string{
		"Ansible":         managed.Ansible,
		"AnsiblePlaybook": managed.AnsiblePlaybook,
		"Python":          managed.Python,
		"CollectionsPath": managed.CollectionsPath,
	} {
		if got == "" || !strings.HasPrefix(got, "/bin/ansible/2.21.0/") {
			t.Errorf("%s = %q, want under /bin/ansible/2.21.0/", name, got)
		}
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
