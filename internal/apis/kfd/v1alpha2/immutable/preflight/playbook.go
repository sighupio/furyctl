// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package preflight

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The OnPremises kind holds a copy of these names and of the function below, in
// internal/apis/kfd/v1alpha2/onpremises/preflight. The two kinds read the same names today,
// and each one keeps its own copy, because the name that a kind runs belongs to that kind.
// A change to one of the two copies needs a look at the other one.
const (
	fetchAdminConfPlaybook = "fetch-admin-conf-playbook.yaml"
	legacyVerifyPlaybook   = "verify-playbook.yaml"
)

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
