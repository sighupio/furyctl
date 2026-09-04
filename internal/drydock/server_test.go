// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package drydock

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
)

func newTestServer(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()

	reg, err := LoadEmbedded()
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "furyctl.yaml")

	distro, err := filepath.Abs("testdata/distro")
	require.NoError(t, err)

	s := NewServer(reg, distro, out, git.ProtocolHTTPS)

	h, err := s.Handler()
	require.NoError(t, err)

	return s, h, out
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func TestWizardsEndpoint(t *testing.T) {
	t.Parallel()

	_, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/api/wizards", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []WizardInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "Immutable", got[0].Kind)
	assert.Equal(t, []string{"v1.35.1"}, got[0].Versions, "a local distro location offers exactly its own version")
}

// The picker asks nobody: the versions are the ones this furyctl supports for the kind, narrowed by
// the range the wizard declares. Today that intersection is a single version.
func TestWizardsEndpointOffersWhatFuryctlSupports(t *testing.T) {
	t.Parallel()

	reg, err := LoadEmbedded()
	require.NoError(t, err)

	s := NewServer(reg, "", filepath.Join(t.TempDir(), "furyctl.yaml"), git.ProtocolHTTPS)

	h, err := s.Handler()
	require.NoError(t, err)

	rec := do(t, h, http.MethodGet, "/api/wizards", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []WizardInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)

	supported, err := distribution.CompatibleVersions("Immutable")
	require.NoError(t, err)
	assert.Subset(t, supported, got[0].Versions, "a wizard covers a subset of what furyctl supports")
	assert.Equal(t, []string{"v1.35.1"}, got[0].Versions)
}

func TestSessionPreviewWrite(t *testing.T) {
	t.Parallel()

	s, h, out := newTestServer(t)

	rec := do(t, h, http.MethodPost, "/api/session", map[string]string{"kind": "Immutable", "version": "v1.35.1"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var w Wizard
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	assert.Equal(t, "Immutable", w.Kind)

	answers := loadAnswers(t)

	rec = do(t, h, http.MethodPost, "/api/preview", map[string]any{"answers": answers})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var preview struct {
		YAML          string       `json:"yaml"`
		Errors        []FieldError `json:"errors"`
		TemplateError string       `json:"templateError"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	assert.Contains(t, preview.YAML, "kind: Immutable")
	assert.Empty(t, preview.Errors)
	assert.Empty(t, preview.TemplateError)

	// The server injects the session version; whatever the client sends is ignored.
	answers["version"] = "v9.9.9"
	rec = do(t, h, http.MethodPost, "/api/preview", map[string]any{"answers": answers})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	assert.Contains(t, preview.YAML, "distributionVersion: v1.35.1")

	rec = do(t, h, http.MethodPost, "/api/write", map[string]any{"answers": answers})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(b), "kind: Immutable")

	select {
	case <-s.Written():
	default:
		t.Fatal("Written channel not closed after a successful write")
	}

	rec = do(t, h, http.MethodPost, "/api/write", map[string]any{"answers": answers})
	assert.Equal(t, http.StatusConflict, rec.Code, "second write must refuse to overwrite")
}

func TestWriteRefusesInvalidDocument(t *testing.T) {
	t.Parallel()

	_, h, out := newTestServer(t)

	rec := do(t, h, http.MethodPost, "/api/session", map[string]string{"kind": "Immutable", "version": "v1.35.1"})
	require.Equal(t, http.StatusOK, rec.Code)

	answers := loadAnswers(t)

	nodes, ok := answers["nodes"].([]any)
	require.True(t, ok)

	lb1, ok := nodes[0].(map[string]any)
	require.True(t, ok)

	lb1["macAddress"] = "nope"

	rec = do(t, h, http.MethodPost, "/api/write", map[string]any{"answers": answers})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	_, err := os.Stat(out)
	assert.True(t, os.IsNotExist(err), "nothing must be written on validation errors")
}

func TestPreviewWithoutSession(t *testing.T) {
	t.Parallel()

	_, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodPost, "/api/preview", map[string]any{"answers": map[string]any{}})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSessionRejectsUnknownWizard(t *testing.T) {
	t.Parallel()

	_, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodPost, "/api/session", map[string]string{"kind": "OnPremises", "version": "v1.35.1"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFileEndpointStaysUnderTheOutputDirectory(t *testing.T) {
	t.Parallel()

	_, h, out := newTestServer(t)
	dir := filepath.Dir(out)

	rec := do(t, h, http.MethodPost, "/api/file", map[string]any{
		"path":    "secrets/etcd-encryption-config.yaml",
		"content": "apiVersion: apiserver.config.k8s.io/v1\n",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	written := filepath.Join(dir, "secrets/etcd-encryption-config.yaml")
	content, err := os.ReadFile(written)
	require.NoError(t, err)
	assert.Contains(t, string(content), "apiserver.config.k8s.io")

	info, err := os.Stat(written)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "these files are usually secrets")

	// An existing file is never replaced by accident.
	rec = do(t, h, http.MethodPost, "/api/file", map[string]any{"path": "secrets/etcd-encryption-config.yaml", "content": "x"})
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = do(t, h, http.MethodPost, "/api/file",
		map[string]any{"path": "secrets/etcd-encryption-config.yaml", "content": "replaced", "overwrite": true})
	assert.Equal(t, http.StatusOK, rec.Code)

	// The permissions are the caller's choice, within reason.
	rec = do(t, h, http.MethodPost, "/api/file", map[string]any{"path": "motd", "content": "hello", "mode": "0644"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	info, err = os.Stat(filepath.Join(dir, "motd"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	for _, mode := range []string{"0044", "4755", "banana", "99"} {
		rec = do(t, h, http.MethodPost, "/api/file", map[string]any{"path": "refused", "content": "x", "mode": mode})
		assert.Equal(t, http.StatusBadRequest, rec.Code, mode)
	}

	// Nothing above the configuration's own directory can be reached.
	for _, path := range []string{"../escape.yaml", "secrets/../../escape.yaml", "/etc/passwd", ""} {
		rec = do(t, h, http.MethodPost, "/api/file", map[string]any{"path": path, "content": "x"})
		assert.Equal(t, http.StatusBadRequest, rec.Code, path)
	}

	_, err = os.Stat(filepath.Join(filepath.Dir(dir), "escape.yaml"))
	assert.True(t, os.IsNotExist(err), "a path above the output directory must not be written")
}

func TestUIIsServed(t *testing.T) {
	t.Parallel()

	_, h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}
