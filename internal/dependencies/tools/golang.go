// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/internal/semver"
	"github.com/sighupio/furyctl/internal/tool/golang"
)

func NewGolang(runner *golang.Runner, version string) *Golang {
	return &Golang{
		arch:    runtime.GOARCH,
		os:      runtime.GOOS,
		version: version,
		checker: &checker{
			regex:  regexp.MustCompile("go version " + semver.Regex),
			runner: runner,
			splitFn: func(version string) []string {
				return strings.Split(version, " ")
			},
			trimFn: func(tokens []string) string {
				return tokens[len(tokens)-1]
			},
		},
	}
}

type Golang struct {
	arch    string
	checker *checker
	os      string
	version string
}

func (*Golang) SupportsDownload() bool {
	return true
}

func (g *Golang) SrcPath() string {
	version := semver.EnsurePrefix(g.version)

	switch g.os {
	case "darwin":
		return fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.tar.gz", version, g.os, g.arch)
	case "windows":
		return fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.zip", version, g.os, g.arch)
	default: // linux
		return fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.tar.gz", version, g.os, g.arch)
	}
}

func (g *Golang) Rename(basePath string) error {
	downloadedFile := filepath.Join(basePath, filepath.Base(g.SrcPath()))
	targetDir := filepath.Join(basePath, "golang")

	switch g.os {
	case "windows":
		// Extract ZIP file
		if err := extractZip(downloadedFile, basePath); err != nil {
			return fmt.Errorf("error extracting go zip: %w", err)
		}
	case "darwin", "linux":
		// Extract tar.gz file
		if err := extractTarGz(downloadedFile, basePath); err != nil {
			return fmt.Errorf("error extracting go tar.gz: %w", err)
		}
	default:
		return fmt.Errorf("unsupported OS: %s", g.os)
	}

	// After extraction, Go creates a "go/" directory
	// Rename it to "golang" for consistency
	extractedPath := filepath.Join(basePath, "go")
	if err := os.Rename(extractedPath, targetDir); err != nil {
		return fmt.Errorf("error renaming go directory: %w", err)
	}

	// Clean up the downloaded archive
	if err := os.Remove(downloadedFile); err != nil {
		return fmt.Errorf("error removing downloaded file: %w", err)
	}

	return nil
}

// Helper function to extract tar.gz files
func extractTarGz(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

// Helper function to extract ZIP files (Windows)
func extractZip(archivePath, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destPath, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Golang) CheckBinVersion() error {
	if err := g.checker.version(g.version); err != nil {
		return fmt.Errorf("go: %w", err)
	}

	return nil
}
