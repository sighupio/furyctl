// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package helmfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/helmfile"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

// When the helm plugin is not yet installed and there is no curl/wget to download it, Init must fail
// with a clear, actionable error instead of letting the install hook panic.
func Test_Init_RequiresDownloaderWhenPluginMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // isolate PATH so curl/wget are not found

	r := helmfile.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), helmfile.Paths{
		Helmfile:   "helmfile",
		WorkDir:    t.TempDir(),
		PluginsDir: t.TempDir(), // empty -> helm-diff not installed
	})

	err := r.Init(filepath.Join(t.TempDir(), "helm"))
	require.ErrorIs(t, err, helmfile.ErrPluginDownloaderMissing)
}

// An already-installed plugin (e.g. pre-installed in an air-gapped bundle) must not trip the downloader
// check, so an offline run needs neither curl/wget nor the network.
func Test_Init_SkipsDownloaderCheckWhenPluginPresent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	pluginsDir := t.TempDir()

	diffBin := filepath.Join(pluginsDir, "helm-diff", "bin", "diff")
	err := os.MkdirAll(filepath.Dir(diffBin), os.FileMode(0o755))
	require.NoError(t, err)

	err = os.WriteFile(diffBin, []byte("stub"), os.FileMode(0o755))
	require.NoError(t, err)

	r := helmfile.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), helmfile.Paths{
		Helmfile:   "helmfile",
		WorkDir:    t.TempDir(),
		PluginsDir: pluginsDir,
	})

	err = r.Init(filepath.Join(t.TempDir(), "helm"))
	require.NotErrorIs(t, err, helmfile.ErrPluginDownloaderMissing,
		"downloader check must be skipped when the plugin is present")
}

func TestHelperProcess(t *testing.T) {
	if len(os.Args) < 3 || os.Args[1] != "-test.run=TestHelperProcess" {
		return
	}

	fmt.Fprint(os.Stdout, "ok")
	os.Exit(0)
}
