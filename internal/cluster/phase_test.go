// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cluster_test

import (
	"strings"
	"testing"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
)

// TestNewOperationPhase_AnsibleSystemFallback verifies that a distribution that does not pin an
// ansible bundle version falls back to the system ansible (transition / backward compatibility).
func TestNewOperationPhase_AnsibleSystemFallback(t *testing.T) {
	t.Parallel()

	op := cluster.NewOperationPhase("/wd/kubernetes", config.KFDTools{}, "/bin")

	if op.AnsiblePlaybookPath != "ansible-playbook" {
		t.Errorf("AnsiblePlaybookPath = %q, want \"ansible-playbook\"", op.AnsiblePlaybookPath)
	}

	if op.AnsiblePythonPath != "" {
		t.Errorf("AnsiblePythonPath = %q, want empty (system fallback)", op.AnsiblePythonPath)
	}

	if got := op.AnsiblePlaybookCmd(); got != "ansible-playbook" {
		t.Errorf("AnsiblePlaybookCmd() = %q, want \"ansible-playbook\"", got)
	}
}

// TestNewOperationPhase_AnsibleBundle verifies that a pinned ansible version resolves to the
// self-contained bundle paths and invocation.
func TestNewOperationPhase_AnsibleBundle(t *testing.T) {
	t.Parallel()

	var tools config.KFDTools
	tools.Common.Ansible.Version = "v0.2.1"

	op := cluster.NewOperationPhase("/wd/kubernetes", tools, "/bin")

	wantPython := "/bin/ansible/v0.2.1/python/bin/python3"
	if op.AnsiblePythonPath != wantPython {
		t.Errorf("AnsiblePythonPath = %q, want %q", op.AnsiblePythonPath, wantPython)
	}

	cmd := op.AnsiblePlaybookCmd()
	for _, want := range []string{
		"ANSIBLE_COLLECTIONS_PATH=",
		"/bin/ansible/v0.2.1/python/bin/python3",
		"/bin/ansible/v0.2.1/python/bin/ansible-playbook",
		"/bin/ansible/v0.2.1/collections",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("AnsiblePlaybookCmd() = %q, missing %q", cmd, want)
		}
	}
}
