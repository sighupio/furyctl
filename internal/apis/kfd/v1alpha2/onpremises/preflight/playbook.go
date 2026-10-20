// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package preflight

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

const (
	fetchAdminConfPlaybook = "fetch-admin-conf-playbook.yaml"
	legacyVerifyPlaybook   = "verify-playbook.yaml"
)

// PlaybookRunner runs a playbook in the folder of the preflight phase. *ansible.Runner is one.
type PlaybookRunner interface {
	Playbook(params ...string) ([]byte, error)
}

// AdminConfPlaybookName returns the playbook that fetches admin.conf.
// Distribution versions released before the playbook renaming provide the legacy name.
func AdminConfPlaybookName(workDir string) (string, error) {
	if _, err := os.Stat(filepath.Join(workDir, fetchAdminConfPlaybook)); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("checking for %s: %w", fetchAdminConfPlaybook, err)
		}

		return legacyVerifyPlaybook, nil
	}

	return fetchAdminConfPlaybook, nil
}

// FetchAdminConf runs the playbook that fetches admin.conf into phaseDir, the working folder of
// runner, and reports whether a control plane host gave it. The caller pings every host first.
//
// The playbook comes from templatesDir, the preflight templates of the distribution, and not from
// phaseDir. The phase folder keeps the files of earlier runs, so it can hold a playbook that the
// current distribution does not give.
//
// The playbook of a distribution released before the renaming fails on purpose when no control
// plane host holds admin.conf, so its failure means that the cluster does not exist. The renamed
// playbook does not fail when admin.conf is absent, so its failure is an error.
func FetchAdminConf(runner PlaybookRunner, phaseDir, templatesDir string) (bool, error) {
	adminConfPath := filepath.Join(phaseDir, "admin.conf")

	// A cluster that the operator removed would otherwise read as a cluster that is there.
	if err := os.Remove(adminConfPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("error removing admin.conf of an earlier run: %w", err)
	}

	playbook, err := AdminConfPlaybookName(templatesDir)
	if err != nil {
		return false, fmt.Errorf("error selecting admin.conf playbook: %w", err)
	}

	if _, err := runner.Playbook(playbook); err != nil {
		if playbook != legacyVerifyPlaybook {
			return false, fmt.Errorf("error fetching admin.conf: %w", err)
		}

		logrus.Debugf("%s: %v", playbook, err)
	}

	if _, err := os.Stat(adminConfPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("error reading the kubeconfig of the cluster: %w", err)
	}

	return true, nil
}
