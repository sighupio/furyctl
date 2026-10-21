// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package ansible

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func TestHostFailure_Cause(t *testing.T) {
	t.Parallel()

	rc := 2

	for _, tc := range []struct {
		f    hostFailure
		want string
	}{
		{hostFailure{Msg: "Task failed: Unexpected AnsibleActionFail error: no file\nmore"}, "no file"},
		{hostFailure{Msg: "Failed to connect to the host via ssh: ssh: timed out"}, "ssh: timed out"},
		{hostFailure{Msg: "non-zero return code", RC: &rc, Stderr: "a\nno such file\n"}, "rc=2: no such file"},
		{hostFailure{Msg: "non-zero return code", RC: &rc}, "non-zero return code"},
	} {
		assert.Equal(t, tc.want, tc.f.cause())
	}
}

func writeFailures(t *testing.T, json string) string {
	t.Helper()

	file := filepath.Join(t.TempDir(), "failures.json")
	require.NoError(t, os.WriteFile(file, []byte(json), 0o600))

	return file
}

func TestDescribe_Playbook(t *testing.T) {
	t.Parallel()

	file := writeFailures(t, `[
		{"host":"zuc3","play":"Set up","task":"Write config","msg":"Destination does not exist"},
		{"host":"zuc1","unreachable":true,"play":"Set up","task":"Ping","msg":"ssh: down"}]`)

	e := &execx.CmdError{Lines: []execx.Line{
		{Text: "[WARNING]: duplicate key", Stderr: true},
		{Text: "  in furyctl.yaml", Stderr: true},
		{Text: "TASK [Write config] ***"},
		{Text: "[DEPRECATION WARNING]: old", Stderr: true},
		{Text: "PLAY RECAP ***"},
		{Text: "zuc1 : failed=0"},
		{Text: "zuc3 : failed=1"},
		{Text: ""},
	}}

	describe(e, "/venv/bin/ansible-playbook", []string{"apply.yaml"}, file)

	assert.Equal(t, "ansible-playbook", e.Name)
	assert.Equal(t, []string{"apply.yaml"}, e.Args)
	assert.Empty(t, e.LogName)
	assert.Equal(t, "ansible failed on 2 host(s):\n"+
		`  zuc1: unreachable at "Set up" > "Ping": ssh: down`+"\n"+
		`  zuc3: failed at "Set up" > "Write config": Destination does not exist`, e.Summary)
	assert.Equal(t, "Recap of the playbook:", e.LeadIn)
	assert.Equal(t, []string{"zuc1 : failed=0", "zuc3 : failed=1"}, e.Block)
	assert.Equal(t, "(2 Ansible warnings not shown, the full output has them)", e.Note)
}

func TestDescribe_Ping(t *testing.T) {
	t.Parallel()

	file := writeFailures(t, `[{"host":"zuc3","unreachable":true,"msg":"Failed to connect to the host via ssh: ssh: down"}]`)

	e := &execx.CmdError{Lines: []execx.Line{{Text: "zuc3 | UNREACHABLE! => {"}}}

	describe(e, "ansible", []string{"all", "-m", "ping"}, file)

	assert.Equal(t, "ansible-ping", e.LogName)
	assert.Equal(t, "these hosts do not answer on ssh:\n  zuc3: ssh: down", e.Summary)
	assert.Empty(t, e.Block)

	// A host that answers but fails is not hidden behind the ssh summary.
	file = writeFailures(t, `[{"host":"a","unreachable":true,"msg":"down"},{"host":"b","task":"ping","msg":"no python"}]`)

	describe(e, "ansible", []string{"all", "-m", "ping"}, file)

	assert.Contains(t, e.Summary, "ansible failed on 2 host(s):")
}

func TestDescribe_NoPlay(t *testing.T) {
	t.Parallel()

	lines := []execx.Line{{Text: "PLAY [x] ***"}}
	for range 12 {
		lines = append(lines, execx.Line{Text: "noise"})
	}

	lines = append(lines,
		execx.Line{Text: "[WARNING]: something", Stderr: true},
		execx.Line{Text: "[ERROR]: The role 'missing' was not found", Stderr: true},
		execx.Line{Text: "Origin: pb.yaml:5:7", Stderr: true},
	)

	e := &execx.CmdError{}
	e.Lines = lines

	describe(e, "ansible-playbook", []string{"pb.yaml"}, filepath.Join(t.TempDir(), "missing.json"))

	assert.Empty(t, e.Summary)
	assert.Equal(t, []string{"[ERROR]: The role 'missing' was not found", "Origin: pb.yaml:5:7"}, e.Block)
	assert.Equal(t, "(1 Ansible warning not shown, the full output has it)", e.Note)
}

func TestModule(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ping", module([]string{"all", "-m", "ping"}))
	assert.Equal(t, "ping", module([]string{"all", "--module-name", "ping"}))
	assert.Equal(t, "ping", module([]string{"all", "--module-name=ping"}))
	assert.Empty(t, module([]string{"all", "-m"}))
	assert.Empty(t, module([]string{"apply.yaml"}))
}
