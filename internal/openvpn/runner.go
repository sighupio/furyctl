// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openvpn

import (
	"fmt"
	"os"
	"os/exec"
)

type Paths struct {
	Secrets string
}

type Runner struct {
	paths Paths
}

func NewRunner(paths Paths) *Runner {
	return &Runner{
		paths: paths,
	}
}

func (r *Runner) newCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = r.paths.Secrets

	return cmd
}

func (r *Runner) Connect(name string) error {
	return r.newCmd("openvpn", "--config", fmt.Sprintf("%s.ovpn", name), "--daemon").Run()
}
