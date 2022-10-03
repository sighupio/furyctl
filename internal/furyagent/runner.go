// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package furyagent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
)

type Paths struct {
	Bin     string
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

func (r *Runner) newCmd(stdOut io.Writer, args ...string) *exec.Cmd {
	cmd := exec.Command(r.paths.Bin, args...)
	cmd.Stdout = stdOut
	cmd.Stderr = os.Stderr
	cmd.Dir = r.paths.Secrets

	return cmd
}

func (r *Runner) ConfigOpenvpnClient(name string) error {
	var outBuffer bytes.Buffer

	out := io.MultiWriter(os.Stdout, &outBuffer)
	cmd := r.newCmd(out, "configure", "openvpn-client", fmt.Sprintf("--client-name=%s", name), "--config=furyagent.yml")
	if err := cmd.Run(); err != nil {
		return err
	}

	return os.WriteFile(path.Join(r.paths.Secrets, fmt.Sprintf("%s.ovpn", name)), outBuffer.Bytes(), 0o600)
}
