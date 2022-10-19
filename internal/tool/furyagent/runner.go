// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package furyagent

import (
	"fmt"
	"os"
	"path"

	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Paths struct {
	Furyagent string
	WorkDir   string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{
		executor: executor,
		paths:    paths,
	}
}

func (r *Runner) ConfigOpenvpnClient(name string) error {
	cmd := execx.NewCmd(r.paths.Furyagent, execx.CmdOptions{
		Args:     []string{"configure", "openvpn-client", fmt.Sprintf("--client-name=%s", name), "--config=furyagent.yml"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	})

	if err := cmd.Run(); err != nil {
		return err
	}

	return os.WriteFile(path.Join(r.paths.WorkDir,
		fmt.Sprintf("%s.ovpn", name)),
		cmd.Log.Out.Bytes(), iox.FullRWPermAccess)
}

func (r *Runner) Version() (string, error) {
	return execx.CombinedOutput(execx.NewCmd(r.paths.Furyagent, execx.CmdOptions{
		Args:     []string{"version"},
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
	}))
}
