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
