package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sighupio/furyctl/internal/semver"
)

func NewFuryAgent(version string) *FuryAgent {
	return &FuryAgent{
		version: version,
		os:      runtime.GOOS,
		arch:    "amd64",
	}
}

type FuryAgent struct {
	version string
	os      string
	arch    string
}

func (f *FuryAgent) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/sighupio/furyagent/releases/download/%s/furyagent-%s-%s",
		semver.EnsurePrefix(f.version, "v"),
		f.os,
		f.arch,
	)
}

func (f *FuryAgent) Rename(basePath string) error {
	oldName := fmt.Sprintf("furyagent-%s-%s", f.os, f.arch)
	newName := "furyagent"

	return os.Rename(filepath.Join(basePath, oldName), filepath.Join(basePath, newName))
}
