// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package preflight_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/preflight"
)

var errPlaybook = errors.New("playbook failed")

// fakeRunner stands for ansible. On success it writes admin.conf when writeAdminConf is set.
type fakeRunner struct {
	dir            string
	writeAdminConf bool
	err            error
	ran            string
}

func (f *fakeRunner) Playbook(params ...string) ([]byte, error) {
	f.ran = params[0]

	if f.err != nil {
		return nil, f.err
	}

	if f.writeAdminConf {
		return nil, os.WriteFile(filepath.Join(f.dir, "admin.conf"), []byte("kubeconfig"), 0o600)
	}

	return nil, nil
}

// FetchAdminConf picks the playbook from the templates of the distribution, reads a failure of the
// legacy playbook as "no cluster", and never reports the admin.conf of an earlier run.
func TestFetchAdminConf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		renamed        bool
		stalePlaybook  bool
		staleAdminConf bool
		writeAdminConf bool
		runErr         error
		wantPlaybook   string
		wantExists     bool
		wantErr        bool
	}{
		{name: "renamed, admin.conf fetched", renamed: true, writeAdminConf: true,
			wantPlaybook: "fetch-admin-conf-playbook.yaml", wantExists: true},
		{name: "renamed, no cluster", renamed: true, wantPlaybook: "fetch-admin-conf-playbook.yaml"},
		{name: "renamed, the playbook fails", renamed: true, runErr: errPlaybook,
			wantPlaybook: "fetch-admin-conf-playbook.yaml", wantErr: true},
		{name: "legacy, the playbook fails on purpose", runErr: errPlaybook, wantPlaybook: "verify-playbook.yaml"},
		{name: "legacy, admin.conf fetched", writeAdminConf: true, wantPlaybook: "verify-playbook.yaml", wantExists: true},
		{name: "admin.conf of an earlier run", renamed: true, staleAdminConf: true,
			wantPlaybook: "fetch-admin-conf-playbook.yaml"},
		{name: "renamed playbook of an earlier run", stalePlaybook: true, runErr: errPlaybook,
			wantPlaybook: "verify-playbook.yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			phaseDir, templatesDir := t.TempDir(), t.TempDir()

			if tc.renamed {
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "fetch-admin-conf-playbook.yaml"), nil, 0o600))
			}

			if tc.stalePlaybook {
				require.NoError(t, os.WriteFile(filepath.Join(phaseDir, "fetch-admin-conf-playbook.yaml"), nil, 0o600))
			}

			if tc.staleAdminConf {
				require.NoError(t, os.WriteFile(filepath.Join(phaseDir, "admin.conf"), []byte("old"), 0o600))
			}

			runner := &fakeRunner{dir: phaseDir, writeAdminConf: tc.writeAdminConf, err: tc.runErr}

			exists, err := preflight.FetchAdminConf(runner, phaseDir, templatesDir)

			assert.Equal(t, tc.wantPlaybook, runner.ran)

			if tc.wantErr {
				require.ErrorIs(t, err, errPlaybook)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantExists, exists)
		})
	}
}
