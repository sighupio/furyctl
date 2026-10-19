// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
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

func (r *Runner) Playbook(params ...string) ([]byte, error) {
	region := execx.NewOutputRegion()
	defer region.Clear()

	return r.run(r.paths.AnsiblePlaybook, params, region, region.Stream())
}

func (r *Runner) Exec(params ...string) ([]byte, error) {
	return r.run(r.paths.Ansible, params, nil, nil)
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

// run runs the given ansible entrypoint with the furyctl_failures callback, so a failure can name the
// failed hosts. It returns stdout.
func (r *Runner) run(entrypoint string, args []string, out, errOut io.Writer) ([]byte, error) {
	dir, err := os.MkdirTemp("", "furyctl-ansible-")
	if err != nil {
		return nil, fmt.Errorf("error creating the ansible callback folder: %w", err)
	}

	osx.RegisterTempDir(dir)

	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "furyctl_failures.py"), failuresPlugin, iox.FullRWPermAccess); err != nil {
		return nil, fmt.Errorf("error writing the ansible callback: %w", err)
	}

	failuresFile := filepath.Join(dir, "failures.json")

	// ANSIBLE_LOAD_CALLBACK_PLUGINS makes the ad-hoc ansible command load the callback too.
	// Keep the callback folders that the user set, after the one of furyctl.
	plugins := dir
	if p := os.Getenv("ANSIBLE_CALLBACK_PLUGINS"); p != "" {
		plugins += string(os.PathListSeparator) + p
	}

	env := []string{
		"ANSIBLE_CALLBACK_PLUGINS=" + plugins,
		"ANSIBLE_LOAD_CALLBACK_PLUGINS=True",
		"FURYCTL_ANSIBLE_FAILURES=" + failuresFile,
	}

	cmd, id := r.build(entrypoint, args, env, out, errOut)
	defer r.deleteCmd(id)

	cmd.OnError = func(e *execx.CmdError) { describe(e, entrypoint, args, failuresFile) }

	if err := cmd.Run(); err != nil {
		return nil, err //nolint:wrapcheck // The error names the command.
	}

	return cmd.Log.Out.Bytes(), nil
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	return r.build(r.paths.Ansible, args, nil, nil, nil)
}

// build runs the given ansible entrypoint directly (host) or as `<python> <entrypoint> ...` with the
// collections env (mise-managed). Non-nil out and errOut also get stdout and stderr.
func (r *Runner) build(
	entrypoint string,
	args, env []string,
	out, errOut io.Writer,
) (*execx.Cmd, string) {
	name := entrypoint
	fullArgs := args

	if r.paths.Python != "" {
		name = r.paths.Python

		fullArgs = append([]string{entrypoint}, args...)

		if r.paths.CollectionsPath != "" {
			env = append(env, "ANSIBLE_COLLECTIONS_PATH="+r.paths.CollectionsPath)
		}
	}

	cmd := execx.NewCmd(name, execx.CmdOptions{
		Args:     fullArgs,
		Env:      env,
		Executor: r.executor,
		Out:      out,
		Err:      errOut,
		WorkDir:  r.paths.WorkDir,
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}
