// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package ansible

import (
	"slices"
	"strings"
	"testing"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func bundleRunner() *Runner {
	return NewRunner(execx.NewStdExecutor(), Paths{
		Python:          "/vendor/bin/ansible/v0.2.0/python/bin/python3",
		Ansible:         "/vendor/bin/ansible/v0.2.0/python/bin/ansible",
		AnsiblePlaybook: "/vendor/bin/ansible/v0.2.0/python/bin/ansible-playbook",
		CollectionsPath: "/vendor/bin/ansible/v0.2.0/collections",
		WorkDir:         "/work",
	})
}

func systemRunner() *Runner {
	return NewRunner(execx.NewStdExecutor(), Paths{
		Ansible:         "ansible",
		AnsiblePlaybook: "ansible-playbook",
		WorkDir:         "/work",
	})
}

// TestRunner_BundleInvocation proves furyctl runs the bundled ansible: ansible-playbook is
// invoked as `python3 <script> <args...>` and CmdPath points at the bundle python.
func TestRunner_BundleInvocation(t *testing.T) {
	r := bundleRunner()

	if got := r.CmdPath(); got != r.paths.Python {
		t.Fatalf("CmdPath = %q, want %q", got, r.paths.Python)
	}

	name, args := r.resolve(r.paths.AnsiblePlaybook, []string{"create-playbook.yaml", "--limit", "node1"})

	if name != r.paths.Python {
		t.Errorf("exec name = %q, want python %q", name, r.paths.Python)
	}

	want := []string{r.paths.AnsiblePlaybook, "create-playbook.yaml", "--limit", "node1"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestRunner_BundleEnv(t *testing.T) {
	env := bundleRunner().bundleEnv()

	if !slices.Contains(env, "ANSIBLE_COLLECTIONS_PATH=/vendor/bin/ansible/v0.2.0/collections") {
		t.Errorf("missing ANSIBLE_COLLECTIONS_PATH in env %v", env)
	}

	var hasPath bool

	for _, e := range env {
		if strings.HasPrefix(e, "PATH=/vendor/bin/ansible/v0.2.0/python/bin") {
			hasPath = true
		}
	}

	if !hasPath {
		t.Errorf("env PATH not prefixed with the bundle bin dir: %v", env)
	}
}

func TestRunner_SystemFallback(t *testing.T) {
	r := systemRunner()

	if got := r.CmdPath(); got != "ansible" {
		t.Errorf("CmdPath = %q, want \"ansible\"", got)
	}

	name, args := r.resolve(r.paths.AnsiblePlaybook, []string{"create-playbook.yaml"})
	if name != "ansible-playbook" {
		t.Errorf("exec name = %q, want \"ansible-playbook\"", name)
	}

	if !slices.Equal(args, []string{"create-playbook.yaml"}) {
		t.Errorf("args = %v, want [create-playbook.yaml]", args)
	}

	if env := r.bundleEnv(); env != nil {
		t.Errorf("bundleEnv = %v, want nil for system fallback", env)
	}
}
