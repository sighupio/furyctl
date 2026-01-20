// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSSHPublicKeys_PrivateKeyPath(t *testing.T) {
	t.Parallel()

	// Create temp directory with test SSH keys.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_test")
	publicKeyPath := privateKeyPath + ".pub"
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC test@example.com"

	if err := os.WriteFile(publicKeyPath, []byte(testPublicKey+"\n"), filePermissionUserReadWrite); err != nil {
		t.Fatalf("failed to write test public key: %v", err)
	}

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config using privateKeyPath (new recommended field).
	sshConfig := map[string]any{
		"username":       "core",
		"privateKeyPath": privateKeyPath,
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err != nil {
		t.Fatalf("ReadSSHPublicKeys() error = %v, want nil", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	if keys[0] != testPublicKey {
		t.Errorf("key mismatch: got %q, want %q", keys[0], testPublicKey)
	}
}

func TestReadSSHPublicKeys_KeyPath_Deprecated(t *testing.T) {
	t.Parallel()

	// Create temp directory with test SSH keys.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_deprecated")
	publicKeyPath := privateKeyPath + ".pub"
	testPublicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@deprecated.com"

	if err := os.WriteFile(publicKeyPath, []byte(testPublicKey+"\n"), filePermissionUserReadWrite); err != nil {
		t.Fatalf("failed to write test public key: %v", err)
	}

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config using keyPath (deprecated field).
	// This should still work but log a warning.
	sshConfig := map[string]any{
		"username": "core",
		"keyPath":  privateKeyPath,
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err != nil {
		t.Fatalf("ReadSSHPublicKeys() with deprecated keyPath error = %v, want nil", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	if keys[0] != testPublicKey {
		t.Errorf("key mismatch: got %q, want %q", keys[0], testPublicKey)
	}

	// Note: Testing the warning log would require capturing log output,
	// which is beyond the scope of this unit test. Integration tests should verify this.
}

func TestReadSSHPublicKeys_BothSpecified(t *testing.T) {
	t.Parallel()

	// Create temp directory with test SSH keys.
	tmpDir := t.TempDir()

	// Create two different key files.
	deprecatedKeyPath := filepath.Join(tmpDir, "id_deprecated")
	deprecatedPublicKeyPath := deprecatedKeyPath + ".pub"
	deprecatedPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC deprecated@example.com"

	newKeyPath := filepath.Join(tmpDir, "id_new")
	newPublicKeyPath := newKeyPath + ".pub"
	newPublicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI new@example.com"

	if err := os.WriteFile(deprecatedPublicKeyPath, []byte(deprecatedPublicKey+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write deprecated public key: %v", err)
	}
	if err := os.WriteFile(newPublicKeyPath, []byte(newPublicKey+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write new public key: %v", err)
	}

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config with BOTH fields specified.
	// PrivateKeyPath should take priority over keyPath.
	sshConfig := map[string]any{
		"username":       "core",
		"keyPath":        deprecatedKeyPath,
		"privateKeyPath": newKeyPath,
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err != nil {
		t.Fatalf("ReadSSHPublicKeys() with both fields error = %v, want nil", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Should use the NEW key (from privateKeyPath), not the deprecated one.
	if keys[0] != newPublicKey {
		t.Errorf("expected privateKeyPath to take priority: got %q, want %q", keys[0], newPublicKey)
	}
}

func TestReadSSHPublicKeys_PublicKeyPath_Explicit(t *testing.T) {
	t.Parallel()

	// Create temp directory with test SSH keys in non-standard location.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "keys", "private_key")
	publicKeyPath := filepath.Join(tmpDir, "public_keys", "public_key.pub")
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC explicit@example.com"

	// Create subdirectories.
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0o755); err != nil {
		t.Fatalf("failed to create private key dir: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(publicKeyPath), 0o755); err != nil {
		t.Fatalf("failed to create public key dir: %v", err)
	}

	if err := os.WriteFile(publicKeyPath, []byte(testPublicKey+"\n"), filePermissionUserReadWrite); err != nil {
		t.Fatalf("failed to write test public key: %v", err)
	}

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config with explicit publicKeyPath (non-standard location).
	sshConfig := map[string]any{
		"username":       "core",
		"privateKeyPath": privateKeyPath,
		"publicKeyPath":  publicKeyPath,
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err != nil {
		t.Fatalf("ReadSSHPublicKeys() with explicit publicKeyPath error = %v, want nil", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	if keys[0] != testPublicKey {
		t.Errorf("key mismatch: got %q, want %q", keys[0], testPublicKey)
	}
}

func TestReadSSHPublicKeys_PublicKeyPath_Derived(t *testing.T) {
	t.Parallel()

	// Create temp directory with test SSH keys.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_derived")
	publicKeyPath := privateKeyPath + ".pub"
	testPublicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI derived@example.com"

	if err := os.WriteFile(publicKeyPath, []byte(testPublicKey+"\n"), filePermissionUserReadWrite); err != nil {
		t.Fatalf("failed to write test public key: %v", err)
	}

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config WITHOUT explicit publicKeyPath.
	// PublicKeyPath should auto-derive as privateKeyPath + ".pub".
	sshConfig := map[string]any{
		"username":       "core",
		"privateKeyPath": privateKeyPath,
		// PublicKeyPath intentionally omitted.
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err != nil {
		t.Fatalf("ReadSSHPublicKeys() with derived publicKeyPath error = %v, want nil", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	if keys[0] != testPublicKey {
		t.Errorf("key mismatch: got %q, want %q", keys[0], testPublicKey)
	}
}

func TestReadSSHPublicKeys_NeitherSpecified(t *testing.T) {
	t.Parallel()

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config without privateKeyPath or keyPath.
	// Error should be returned.
	sshConfig := map[string]any{
		"username": "core",
		// Both privateKeyPath and keyPath omitted.
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err == nil {
		t.Fatal("ReadSSHPublicKeys() without key paths should return error, got nil")
	}

	if keys != nil {
		t.Errorf("expected nil keys on error, got %v", keys)
	}

	expectedErrMsg := "either ssh.privateKeyPath or ssh.keyPath (deprecated) must be specified"
	if err.Error() != expectedErrMsg {
		t.Errorf("error message mismatch: got %q, want %q", err.Error(), expectedErrMsg)
	}
}

func TestReadSSHPublicKeys_EmptyPublicKeyFile(t *testing.T) {
	t.Parallel()

	// Create temp directory with empty public key file.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_empty")
	publicKeyPath := privateKeyPath + ".pub"

	// Create empty public key file.
	if err := os.WriteFile(publicKeyPath, []byte("   \n  \n"), 0o600); err != nil {
		t.Fatalf("failed to write empty public key: %v", err)
	}

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	sshConfig := map[string]any{
		"username":       "core",
		"privateKeyPath": privateKeyPath,
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err == nil {
		t.Fatal("ReadSSHPublicKeys() with empty public key should return error, got nil")
	}

	if keys != nil {
		t.Errorf("expected nil keys on error, got %v", keys)
	}

	if !contains(err.Error(), "is empty") {
		t.Errorf("error should mention empty file: %v", err)
	}
}

func TestReadSSHPublicKeys_PublicKeyFileNotFound(t *testing.T) {
	t.Parallel()

	// Create temp directory without creating the public key file.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_nonexistent")
	// PublicKeyPath will be derived but file doesn't exist.

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	sshConfig := map[string]any{
		"username":       "core",
		"privateKeyPath": privateKeyPath,
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err == nil {
		t.Fatal("ReadSSHPublicKeys() with non-existent public key should return error, got nil")
	}

	if keys != nil {
		t.Errorf("expected nil keys on error, got %v", keys)
	}

	if !contains(err.Error(), "error reading SSH public key") {
		t.Errorf("error should mention reading public key: %v", err)
	}
}

func TestReadSSHPublicKeys_EnvVarExpansion(t *testing.T) {
	// Note: Cannot use t.Parallel() because t.Setenv() modifies global state.

	// Create temp directory with test SSH keys.
	tmpDir := t.TempDir()
	privateKeyPath := filepath.Join(tmpDir, "id_envtest")
	publicKeyPath := privateKeyPath + ".pub"
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC envtest@example.com"

	if err := os.WriteFile(publicKeyPath, []byte(testPublicKey+"\n"), filePermissionUserReadWrite); err != nil {
		t.Fatalf("failed to write test public key: %v", err)
	}

	// Set custom environment variable for testing.
	t.Setenv("TEST_SSH_DIR", tmpDir)

	// Create Infrastructure instance.
	infra := &Infrastructure{}

	// Test config using environment variable in path.
	sshConfig := map[string]any{
		"username":       "core",
		"privateKeyPath": "${TEST_SSH_DIR}/id_envtest",
	}

	keys, err := infra.readSSHPublicKeys(sshConfig)
	if err != nil {
		t.Fatalf("ReadSSHPublicKeys() with env var expansion error = %v, want nil", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	if keys[0] != testPublicKey {
		t.Errorf("key mismatch: got %q, want %q", keys[0], testPublicKey)
	}
}

// Helper function to check if string contains substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
