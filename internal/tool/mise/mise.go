// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mise bundles the mise (jdx/mise) tool manager: it pins a mise version, knows how to build
// the per-platform download URL + checksum, and exposes a hermetic Runner that drives our bundled
// mise in complete isolation from any mise the user may have installed on their system.
package mise

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

// Version is the pinned mise release furyctl bundles. It is owned by furyctl (the tool-management
// engine), NOT by the distribution, to avoid a circular dependency with fury-distribution.
const (
	Version = "v2026.6.14"

	// File mode for the bundled mise binary (owner rwx, group/other rx).
	binPerm = 0o755
)

var (
	ErrUnsupportedPlatform = errors.New("unsupported os/arch for mise")
	errEmptyPath           = errors.New("mise returned an empty path")

	// Checksums of the mise binary per furyctl "<os>/<arch>" (from the release SHASUMS256.txt of
	// Version). Linux uses the static musl build for portability across distros.
	//nolint:gochecknoglobals // lookup table for the pinned release.
	binChecksums = map[string]string{
		"linux/amd64":  "f9ef55de0a8bc2e1290837d9d502347b80aec387c8d5f32c39dd5288c97ba366",
		"linux/arm64":  "4862659710d8750d845e152842134a447e8aebaa657c27b4d09a589ffaed2736",
		"darwin/amd64": "4afa2119311abfff0b1c8ff55fbd55004f1d1b54f3cb02f69d21b73ce9f92e7f",
		"darwin/arm64": "2f70a4df7f12d50226c34b857821c80f911b382598cf3f7972e9d0dee0706bb9",
	}
)

// AssetName maps furyctl os/arch to the mise release asset (single static binary) file name.
func AssetName(goos, goarch string) (string, error) {
	miseOS := map[string]string{"linux": "linux", "darwin": "macos"}[goos]
	miseArch := map[string]string{"amd64": "x64", "arm64": "arm64"}[goarch]

	if miseOS == "" || miseArch == "" {
		return "", fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}

	name := fmt.Sprintf("mise-%s-%s-%s", Version, miseOS, miseArch)
	if goos == "linux" {
		name += "-musl"
	}

	return name, nil
}

// DownloadURL returns the mise binary URL for the platform, with a go-getter checksum query so the
// download is verified on fetch.
func DownloadURL(goos, goarch string) (string, error) {
	name, err := AssetName(goos, goarch)
	if err != nil {
		return "", err
	}

	sum, ok := binChecksums[goos+"/"+goarch]
	if !ok {
		return "", fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}

	return fmt.Sprintf(
		"https://github.com/jdx/mise/releases/download/%s/%s?checksum=sha256:%s",
		Version, name, sum,
	), nil
}

// Downloader is the minimal client EnsureBinary needs to fetch the mise binary.
type Downloader interface {
	Download(src, dst string) error
}

// EnsureBinary downloads (once) the bundled mise binary for the host platform into
// binDir/mise/<Version>/mise (checksum-verified via the go-getter `?checksum=` query) and returns
// its path. Idempotent: if the binary already exists it is reused.
func EnsureBinary(client Downloader, binDir string) (string, error) {
	dstDir := filepath.Join(binDir, "mise", Version)
	final := filepath.Join(dstDir, "mise")

	if _, err := os.Stat(final); err == nil {
		return final, nil
	}

	url, err := DownloadURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if err := client.Download(url, dstDir); err != nil {
		return "", fmt.Errorf("error downloading mise: %w", err)
	}

	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if err := os.Rename(filepath.Join(dstDir, asset), final); err != nil {
		return "", fmt.Errorf("error renaming mise binary: %w", err)
	}

	if err := os.Chmod(final, binPerm); err != nil {
		return "", fmt.Errorf("error setting mise executable: %w", err)
	}

	return final, nil
}

// Paths configures the hermetic Runner. DataDir/CacheDir/ConfigFile live under the cluster vendor
// dir so the whole tool set is self-contained (and air-gapped-transferable); WorkDir is an isolated
// empty directory used as CWD so mise never discovers ambient (user/project) config files.
type Paths struct {
	Mise       string
	DataDir    string
	CacheDir   string
	ConfigFile string
	WorkDir    string
}

type Runner struct {
	executor execx.Executor
	paths    Paths
}

func NewRunner(executor execx.Executor, paths Paths) *Runner {
	return &Runner{executor: executor, paths: paths}
}

// Install installs all tools declared in the (hermetic) global config into DataDir, teeing mise's
// progress output to progress (may be nil). No-op if they are already installed.
func (r *Runner) Install(progress io.Writer) error {
	if _, err := execx.CombinedOutput(r.newCmdOut(progress, "install")); err != nil {
		return fmt.Errorf("error running mise install: %w", err)
	}

	return nil
}

// Which resolves the absolute path of a tool's binary (e.g. "kubectl", "tofu", "furyagent").
func (r *Runner) Which(bin string) (string, error) {
	cmd := r.newCmd("which", bin)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error running mise which %s: %w", bin, err)
	}

	path := strings.TrimSpace(cmd.Log.Out.String())
	if path == "" {
		return "", fmt.Errorf("error running mise which %s: %w", bin, errEmptyPath)
	}

	return path, nil
}

// Env returns the environment variables mise would set to activate the configured tools (PATH to the
// tool bins + any tool-specific vars). Used as a base env for executing tools that need their own
// environment (e.g. python-based ones). Returned as "KEY=VALUE" entries.
func (r *Runner) Env() ([]string, error) {
	cmd := r.newCmd("env", "--json")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error running mise env: %w", err)
	}

	vars, err := parseEnvJSON(cmd.Log.Out.String())
	if err != nil {
		return nil, err
	}

	return vars, nil
}

func (r *Runner) Version() (string, error) {
	out, err := execx.CombinedOutput(r.newCmd("--version"))
	if err != nil {
		return "", fmt.Errorf("error running mise --version: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// hermeticEnv isolates our mise from the user's: dedicated data/cache/config, auto-confirm and a
// trusted config path (no prompt).
func (r *Runner) hermeticEnv() []string {
	return []string{
		"MISE_DATA_DIR=" + r.paths.DataDir,
		"MISE_CACHE_DIR=" + r.paths.CacheDir,
		"MISE_GLOBAL_CONFIG_FILE=" + r.paths.ConfigFile,
		"MISE_TRUSTED_CONFIG_PATHS=" + filepath.Dir(r.paths.ConfigFile),
		"MISE_YES=1",
	}
}

// newCmd builds a mise invocation: always `--cd <isolated workdir>` so config discovery can't pick
// up ambient mise.toml files, with the hermetic env.
func (r *Runner) newCmd(args ...string) *execx.Cmd {
	return r.newCmdOut(nil, args...)
}

// newCmdOut is like newCmd but also tees mise's stdout/stderr to progress (in addition to the
// captured buffers), so the caller can stream install output live.
func (r *Runner) newCmdOut(progress io.Writer, args ...string) *execx.Cmd {
	fullArgs := append([]string{"--cd", r.paths.WorkDir}, args...)

	return execx.NewCmd(r.paths.Mise, execx.CmdOptions{
		Args:     fullArgs,
		Env:      r.hermeticEnv(),
		Executor: r.executor,
		Out:      progress,
		Err:      progress,
	})
}
