// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create //nolint:testpackage // exercises the unexported verifySHA256 helper.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "artifact.raw")
	data := []byte("sysext bytes")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])

	if err := verifySHA256(path, want); err != nil {
		t.Errorf("matching digest: got %v, want nil", err)
	}

	if err := verifySHA256(path, "deadbeef"); !errors.Is(err, ErrSysextChecksumMismatch) {
		t.Errorf("wrong digest: got %v, want ErrSysextChecksumMismatch", err)
	}
}
