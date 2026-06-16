// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

type Paths struct {
	Ansible         string
	AnsiblePlaybook string
	WorkDir         string
	// Python and CollectionsPath are set when using a self-contained ansible bundle. When
	// Python is set, ansible/ansible-playbook are invoked as `python3 <script> ...` (so the
	// relocated bundle ignores the build-time shebang) and ANSIBLE_COLLECTIONS_PATH/PATH are
	// injected. When empty, the runner falls back to the system ansible on PATH.
	Python          string
	CollectionsPath string
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
	if r.paths.Python != "" {
		return r.paths.Python
	}

	return r.paths.Ansible
}

// resolve returns the executable and args to run a bundled or system ansible entrypoint.
// With a bundle, the python interpreter is the executable and the console script is its first
// argument (bypassing the broken shebang of a relocated bundle).
func (r *Runner) resolve(script string, args []string) (string, []string) {
	if r.paths.Python == "" {
		return script, args
	}

	return r.paths.Python, append([]string{script}, args...)
}

// bundleEnv injects the env needed by a bundled ansible (collections path + the bundle bin on
// PATH). Returns nil for the system-ansible fallback so the process env is inherited unchanged.
func (r *Runner) bundleEnv() []string {
	if r.paths.Python == "" {
		return nil
	}

	env := []string{}

	if r.paths.CollectionsPath != "" {
		env = append(env, "ANSIBLE_COLLECTIONS_PATH="+r.paths.CollectionsPath)
	}

	env = append(env, "PATH="+filepath.Dir(r.paths.Python)+string(os.PathListSeparator)+os.Getenv("PATH"))

	return env
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	name, cmdArgs := r.resolve(r.paths.Ansible, args)

	cmd := execx.NewCmd(name, execx.CmdOptions{
		Args:     cmdArgs,
		Env:      r.bundleEnv(),
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) newPlaybookCmd(args []string) (*execx.Cmd, string) {
	name, cmdArgs := r.resolve(r.paths.AnsiblePlaybook, args)

	cmd := execx.NewCmd(name, execx.CmdOptions{
		Args:     cmdArgs,
		Env:      r.bundleEnv(),
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
