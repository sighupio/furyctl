// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package preflight_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/preflight"
)

// Both the Immutable and the OnPremises kinds read this name. A version of the
// distribution released before the rename gives the legacy playbook only.
func TestAdminConfPlaybookName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		present []string
		want    string
	}{
		{"current distribution", []string{"fetch-admin-conf-playbook.yaml"}, "fetch-admin-conf-playbook.yaml"},
		{"older distribution", []string{"verify-playbook.yaml"}, "verify-playbook.yaml"},
		{"both names present", []string{"fetch-admin-conf-playbook.yaml", "verify-playbook.yaml"}, "fetch-admin-conf-playbook.yaml"},
		{"neither name present", nil, "verify-playbook.yaml"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for _, f := range test.present {
				require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("---\n"), 0o600))
			}

			got, err := preflight.AdminConfPlaybookName(dir)

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
