package tools

import (
	"fmt"
	"runtime"

	"github.com/sighupio/furyctl/internal/semver"
)

func NewKustomize(version string) *Kustomize {
	return &Kustomize{
		version: version,
		os:      runtime.GOOS,
		arch:    "amd64",
	}
}

type Kustomize struct {
	version string
	os      string
	arch    string
}

func (k *Kustomize) SrcPath() string {
	return fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/%s/kustomize_%s_%s_%s.tar.gz",
		semver.EnsurePrefix(k.version, "v"),
		semver.EnsurePrefix(k.version, "v"),
		k.os,
		k.arch,
	)
}

func (f *Kustomize) Rename(basePath string) error {
	return nil
}
