// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package helmfile

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

// ErrPluginDownloaderMissing is returned when a helm plugin must be installed but neither curl nor wget
// is available to download it.
var ErrPluginDownloaderMissing = errors.New(
	"installing helm plugins (helm-diff) requires 'curl' or 'wget' on the PATH; install one and retry",
)

type Paths struct {
	Helmfile   string
	WorkDir    string
	PluginsDir string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
	cmds     map[string]*execx.Cmd
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	if paths.PluginsDir != "" {
		if err := os.Setenv("HELM_PLUGINS", paths.PluginsDir); err != nil {
			logrus.Fatal(err)
		}
	}

	return &Runner{
		executor: executor,
		paths:    paths,
		cmds:     make(map[string]*execx.Cmd),
	}
}

func (r *Runner) CmdPath() string {
	return r.paths.Helmfile
}

func (r *Runner) newCmd(args []string) (*execx.Cmd, string) {
	cmd := execx.NewCmd(r.paths.Helmfile, execx.CmdOptions{
		Args:     args,
		Executor: r.executor,
		WorkDir:  r.paths.WorkDir,
		// Disable helmfile's "newer version available" check: it hits the network on every
		// invocation (including `version`), which slows things down and breaks air-gapped runs.
		Env: []string{"HELMFILE_UPGRADE_NOTICE_DISABLED=1"},
	})

	id := uuid.NewString()
	r.cmds[id] = cmd

	return cmd, id
}

func (r *Runner) deleteCmd(id string) {
	delete(r.cmds, id)
}

func (r *Runner) Init(helmBinary string) error {
	// Helm plugin install hooks (e.g. helm-diff's install-binary.sh) call `helm` by bare name, so put the
	// helm binary's dir on PATH — furyctl otherwise drives tools by absolute path and there may be no
	// system helm. Set via the process env so it is the single authoritative PATH (execx appends a cmd's
	// Env after os.Environ(), which would leave a duplicate PATH with platform-dependent resolution).
	if err := os.Setenv("PATH", filepath.Dir(helmBinary)+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return fmt.Errorf("error preparing PATH for helm plugins: %w", err)
	}

	// The install hook downloads the plugin binary with curl/wget. Require one — but only when the plugin
	// is not already present: an air-gapped bundle pre-installs it, so an offline run needs neither the
	// downloader nor the network.
	if r.paths.PluginsDir != "" {
		diffBin := filepath.Join(r.paths.PluginsDir, "helm-diff", "bin", "diff")
		if _, err := os.Stat(diffBin); err != nil && !hasDownloader() {
			return ErrPluginDownloaderMissing
		}
	}

	args := []string{"init", "--force", "--helm-binary", helmBinary}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error running helmfile init: %w", err)
	}

	return nil
}

// hasDownloader reports whether curl or wget is available to download helm plugin binaries.
func hasDownloader() bool {
	for _, tool := range []string{"curl", "wget"} {
		if _, err := exec.LookPath(tool); err == nil {
			return true
		}
	}

	return false
}

func (r *Runner) Apply() error {
	args := []string{"apply"}

	cmd, id := r.newCmd(args)
	defer r.deleteCmd(id)

	if _, err := execx.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("error running helmfile apply: %w", err)
	}

	return nil
}

func (r *Runner) Version() (string, error) {
	cmd, id := r.newCmd([]string{"version", "-o=short"})
	defer r.deleteCmd(id)

	out, err := execx.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting helmfile version: %w", err)
	}

	return out, nil
}

func (r *Runner) Stop() error {
	for _, cmd := range r.cmds {
		if err := cmd.Stop(); err != nil {
			return fmt.Errorf("error stopping helmfile runner: %w", err)
		}
	}

	return nil
}
