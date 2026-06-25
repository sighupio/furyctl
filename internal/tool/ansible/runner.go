// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Ansible         string
	AnsiblePlaybook string
	WorkDir         string
	// Python and CollectionsPath are set only for the mise-managed ansible: commands are then run as
	// `<Python> <entrypoint> ...` (bypassing the venv shebang, so it survives air-gapped relocation)
	// with ANSIBLE_COLLECTIONS_PATH pointing at the bundled collections. When empty, the host ansible is
	// invoked by name (backward compatible).
	Python          string
	CollectionsPath string
}

// PathsForVersion derives the runner Paths for the mise-managed ansible materialized under
// <binPath>/ansible/<version>/. When version is empty (distribution does not pin ansible) it falls back
// to the host ansible (bare command names), preserving backward compatibility.
func PathsForVersion(binPath, version, workDir string) Paths {
	if version == "" {
		return Paths{Ansible: "ansible", AnsiblePlaybook: "ansible-playbook", WorkDir: workDir}
	}

	base := filepath.Join(binPath, "ansible", version)
	venvBin := filepath.Join(base, "venv", "bin")

	return Paths{
		Ansible:         filepath.Join(venvBin, "ansible"),
		AnsiblePlaybook: filepath.Join(venvBin, "ansible-playbook"),
		Python:          filepath.Join(venvBin, "python"),
		CollectionsPath: filepath.Join(base, "collections"),
		WorkDir:         workDir,
	}
}

type Runner struct {
	executor execx.Executor
	paths    Paths
	cmds     map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
		cmds:     make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Ansible
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	return r.build(r.paths.Ansible, args)
}

func (r *Runner) newPlaybookCmd(args []string) (*execx.Cmd, string) {
	return r.build(r.paths.AnsiblePlaybook, args)
}

// build runs the given ansible entrypoint directly (host) or as `<python> <entrypoint> ...` with the
// collections env (mise-managed).
func (r *Runner) build(entrypoint string, args []string) (*execx.Cmd, string) {
	name := entrypoint
	fullArgs := args

	var env []string

	if r.paths.Python != "" {
		name = r.paths.Python

		fullArgs = append([]string{entrypoint}, args...)

		if r.paths.CollectionsPath != "" {
			env = []string{"ANSIBLE_COLLECTIONS_PATH=" + r.paths.CollectionsPath}
		}
	}

	cmd := execx.NewCmd(name, execx.CmdOptions{
		Args:     fullArgs,
		Env:      env,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) Playbook(params ...string) ([]byte, error) {
	args := []string{}
	out := []byte{}

	if len(params) > 0 {
		args = append(args, params...)
	}

	cmd, id := r.newPlaybookCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return out, fmt.Errorf("command execution failed: %w", err)
	}

	out = cmd.Log.Out.Bytes()

	return out, nil
}

func (r *Runner) Exec(params ...string) ([]byte, error) {
	args := []string{}
	out := []byte{}

	if len(params) > 0 {
		args = append(args, params...)
	}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if err := cmd.Run(); err != nil {
		return out, fmt.Errorf("command execution failed: %w", err)
	}

	out = cmd.Log.Out.Bytes()

	return out, nil
}

func (r *Runner) Version() (string, error) {
	args := []string{"--version"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting ansible version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping ansible runner: %w", err)
		}
	}

	return nil
}
