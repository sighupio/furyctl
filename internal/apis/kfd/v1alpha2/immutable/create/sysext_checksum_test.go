// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

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

	if err := verifySHA256(path, "deadbeef"); !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("wrong digest: got %v, want ErrChecksumMismatch", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("wrong digest: the file is still on disk (stat error: %v)", err)
	}
}
