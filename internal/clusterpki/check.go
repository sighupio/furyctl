// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterpki

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	parserx "github.com/sighupio/furyctl/internal/parser"
	"github.com/sighupio/furyctl/pkg/template/mapper"
)

var (
	// ErrPathEmpty reports a configuration field that is absent, or that holds an empty string. The
	// schemas of the supported SD versions reject both, so this occurs only with an older schema.
	ErrPathEmpty = errors.New("the configuration field has no value")

	// ErrPathNotAFolder reports a PKI folder path that exists and is not a folder.
	ErrPathNotAFolder = errors.New("the PKI folder path exists and is not a folder")

	// ErrPathNotResolvable reports a relative path without a "./" or "../" prefix. The template mapper
	// does not make such a value absolute, so the playbooks read it from the phase folder.
	ErrPathNotResolvable = errors.New(
		"the PKI folder path is relative without a \"./\" prefix. furyctl cannot resolve such a value. " +
			"The playbooks then read it from a folder inside .furyctl/<cluster>/, and that folder is not the " +
			"same for each phase. Write \"./<path>\" for a folder next to the configuration file, or write " +
			"an absolute path",
	)
)

// controlPlaneFiles returns the files that the kube-control-plane role of the installer copies to each
// control plane node.
func controlPlaneFiles() []string {
	return []string{
		ControlPlaneCaCrt,
		ControlPlaneCaKey,
		ControlPlaneFProxyCrt,
		ControlPlaneFProxyKey,
		ControlPlaneSaKey,
		ControlPlaneSaPub,
	}
}

// etcdFiles returns the files that the etcd role of the installer copies to each etcd node.
func etcdFiles() []string {
	return []string{
		EtcdCaCrt,
		EtcdCaKey,
	}
}

// FolderMissingError reports a PKI folder that does not exist.
type FolderMissingError struct {
	Path string
}

func (e *FolderMissingError) Error() string {
	return fmt.Sprintf(
		"the PKI folder %s does not exist: the Kubernetes and etcd playbooks read the CA certificates "+
			"and keys from it. To create it, run `furyctl create pki --path %s`",
		e.Path,
		e.Path,
	)
}

// FilesMissingError reports a PKI folder that does not hold all the files that the playbooks read.
type FilesMissingError struct {
	Path    string
	Missing []string
}

func (e *FilesMissingError) Error() string {
	return fmt.Sprintf(
		"the PKI folder %s is not complete. These files are absent or empty: %s. Restore them from your "+
			"backup. furyctl does not write to a PKI folder that exists. If the cluster does not exist yet, "+
			"delete the folder and run `furyctl create pki --path %s`",
		e.Path,
		strings.Join(e.Missing, ", "),
		e.Path,
	)
}

// ResolvePath returns the absolute PKI folder path for a value of a configuration file. It applies the
// rules that the template mapper applies to the same value before it renders the playbooks. See
// mapper.Mapper.injectDynamicValuesAndPathsString. An absolute path does not change. A path that starts
// with "./" or "../" resolves against the folder of the configuration file. Any other relative path
// stays relative in the rendered playbook, so this function rejects it.
//
// The result is always absolute, because the messages name the folder and give the command that creates
// it. `furyctl validate config` does not make the path of the configuration file absolute. A relative
// path here would give a message that is correct in one working folder only.
func ResolvePath(value, confDir string) (string, error) {
	if value == "" {
		return "", ErrPathEmpty
	}

	absConfDir, err := filepath.Abs(confDir)
	if err != nil {
		return "", fmt.Errorf("error while getting the absolute path of the configuration folder: %w", err)
	}

	// The mapper expands the dynamic values of the configuration, for example "{env://PKI_DIR}" and
	// "{path://pki}", before it applies the rules for a relative path. The same order is necessary here.
	// Without it, this check rejects a value that the playbooks resolve.
	expanded, err := parserx.NewConfigParser(absConfDir).ParseMultipleDynamicValues(value)
	if err != nil {
		return "", fmt.Errorf("error while expanding the dynamic values of the PKI folder path: %w", err)
	}

	if expanded == "" {
		return "", ErrPathEmpty
	}

	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}

	if !mapper.RelativePathRegexp.MatchString(expanded) {
		return "", fmt.Errorf("%w: %s", ErrPathNotResolvable, expanded)
	}

	return filepath.Join(absConfDir, filepath.Clean(expanded)), nil
}

// Check reports whether absPath holds the master and etcd folders with the files that the installer
// roles read. It returns a *FolderMissingError when the folder is absent, and ErrPathNotAFolder when the
// path is not a folder. It returns a *FilesMissingError when one file or more is absent or empty. It
// returns a read error when furyctl cannot read the folder or a file.
func Check(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &FolderMissingError{Path: absPath}
		}

		return fmt.Errorf("error while checking the PKI folder %s: %w", absPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrPathNotAFolder, absPath)
	}

	controlPlaneMissing, err := missingFiles(absPath, ControlPlanePath, controlPlaneFiles())
	if err != nil {
		return err
	}

	etcdMissing, err := missingFiles(absPath, etcdPath, etcdFiles())
	if err != nil {
		return err
	}

	missing := slices.Concat(controlPlaneMissing, etcdMissing)

	if len(missing) > 0 {
		return &FilesMissingError{Path: absPath, Missing: missing}
	}

	return nil
}

// missingFiles returns the files of dir that are absent, empty, or not a regular file. The returned
// names hold the folder, for example "master/ca.crt", so that the message points at the file. A file
// that furyctl cannot read gives an error, and not a name in the list. The message for an absent file
// tells the user to delete the folder. That step destroys the CA of a cluster that exists.
func missingFiles(absPath, dir string, files []string) ([]string, error) {
	missing := []string{}

	for _, file := range files {
		path := filepath.Join(absPath, dir, file)

		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, filepath.Join(dir, file))

				continue
			}

			return nil, fmt.Errorf("error while reading the PKI file %s: %w", path, err)
		}

		if !info.Mode().IsRegular() || info.Size() == 0 {
			missing = append(missing, filepath.Join(dir, file))
		}
	}

	return missing, nil
}
