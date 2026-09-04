// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package drydock

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	distroconf "github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	//go:embed ui
	uiFS embed.FS

	ErrNoSession     = errors.New("no session: choose a kind and a version first")
	ErrOutputExists  = errors.New("output file already exists")
	ErrFileOutside   = errors.New("a file can only be created next to the configuration")
	ErrFileExists    = errors.New("file already exists")
	ErrUnsupported   = errors.New("version not supported by this furyctl")
	ErrEmbeddedUI    = errors.New("embedded ui not found")
	errDecodeRequest = errors.New("decoding request")
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
	maxBodyBytes      = 8 << 20 // Answers with many nodes and inline files stay far below this.
	outputFileMode    = 0o600
	outputDirMode     = 0o700
)

type session struct {
	wizard     *Wizard
	tpl        string
	version    string
	schemaPath string
}

// Server holds one wizard session at a time. It runs on loopback for one person.
type Server struct {
	reg            *Registry
	distroLocation string
	outputPath     string
	gitProtocol    git.Protocol

	mu       sync.Mutex
	sess     *session
	written  chan struct{}
	once     sync.Once
	releases []string // Release tags known for the picker, resolved once.
	relOnce  sync.Once
}

func NewServer(reg *Registry, distroLocation, outputPath string, gitProtocol git.Protocol) *Server {
	return &Server{
		reg:            reg,
		distroLocation: distroLocation,
		outputPath:     outputPath,
		gitProtocol:    gitProtocol,
		written:        make(chan struct{}),
	}
}

// Written is closed after the first successful write, so the run loop can stop.
func (s *Server) Written() <-chan struct{} {
	return s.written
}

// Handler routes the JSON API under /api/ and serves the embedded UI for everything else.
func (s *Server) Handler() (http.Handler, error) {
	ui, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbeddedUI, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/wizards", s.handleWizards)
	mux.HandleFunc("POST /api/session", s.handleSession)
	mux.HandleFunc("POST /api/preview", s.handlePreview)
	mux.HandleFunc("POST /api/write", s.handleWrite)
	mux.HandleFunc("POST /api/file", s.handleFile)
	// The UI ships inside the binary: a newer furyctl must never be served stale files from the
	// browser cache, and embedded files carry no modification time for the cache to key on.
	files := http.FileServer(http.FS(ui))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))

	return mux, nil
}

func (s *Server) handleWizards(w http.ResponseWriter, _ *http.Request) {
	infos := s.reg.List()

	for i := range infos {
		infos[i].Versions = s.versionsFor(infos[i])
	}

	writeJSON(w, http.StatusOK, infos)
}

// versionsFor lists the releases a wizard can be used with, newest first. With a local
// --distro-location there is exactly one candidate, the version of that checkout; otherwise
// the GitHub releases are filtered by the wizard range and by furyctl's own compatibility.
// When nothing can be listed, the wizard default is offered so the picker never comes empty.
func (s *Server) versionsFor(info WizardInfo) []string {
	wizard, _, err := s.reg.Find(info.Kind, info.DefaultVersion)
	if err != nil {
		return []string{info.DefaultVersion}
	}

	var out []string

	for _, v := range s.knownReleases() {
		if !wizard.InRange(v) {
			continue
		}

		if checker, err := distribution.NewCompatibilityChecker(v, info.Kind); err != nil || !checker.IsCompatible() {
			continue
		}

		out = append(out, v)
	}

	if len(out) == 0 {
		return []string{info.DefaultVersion}
	}

	return out
}

func (s *Server) knownReleases() []string {
	s.relOnce.Do(func() {
		if s.distroLocation != "" {
			manifest, err := yamlx.FromFileV3[distroconf.KFD](s.distroLocation + "/kfd.yaml")
			if err == nil && manifest.Version != "" {
				s.releases = []string{semver.EnsurePrefix(manifest.Version)}
			}

			return
		}

		releases, err := distribution.GetSupportedVersions(git.NewGitHubClient())
		if err != nil {
			logrus.Warnf("cannot list distribution releases, offering the wizard default only: %v", err)

			return
		}

		for _, r := range releases {
			s.releases = append(s.releases, semver.EnsurePrefix(r.Version.String()))
		}
	})

	return s.releases
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"`
		Version string `json:"version"`
	}

	if !readJSON(w, r, &req) {
		return
	}

	wizard, tpl, err := s.reg.Find(req.Kind, req.Version)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrNoWizard) {
			status = http.StatusNotFound
		}

		writeError(w, status, err)

		return
	}

	checker, err := distribution.NewCompatibilityChecker(req.Version, req.Kind)
	if err != nil || !checker.IsCompatible() {
		writeError(w, http.StatusBadRequest, fmt.Errorf("%w: %s %s", ErrUnsupported, req.Kind, req.Version))

		return
	}

	minimalConf := distroconf.Furyctl{
		APIVersion: distribution.APIVersionV1Alpha2,
		Kind:       req.Kind,
		Metadata:   distroconf.FuryctlMeta{Name: "drydock"},
		Spec:       distroconf.FuryctlSpec{DistributionVersion: req.Version},
	}

	logrus.Infof("Downloading distribution %s...", req.Version)

	dl := dist.NewDownloader(netx.NewGoGetterClient(), s.gitProtocol, "")

	res, err := dl.DoDownload(s.distroLocation, minimalConf)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("downloading distribution: %w", err))

		return
	}

	schemaPath, err := distribution.GetPublicSchemaPath(res.RepoPath, minimalConf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("locating schema: %w", err))

		return
	}

	s.mu.Lock()
	s.sess = &session{wizard: wizard, tpl: tpl, version: req.Version, schemaPath: schemaPath}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, wizard)
}

type renderResult struct {
	YAML          string       `json:"yaml"`
	Errors        []FieldError `json:"errors"`
	TemplateError string       `json:"templateError,omitempty"`
}

// render runs the session template on the answers and validates the result. The
// distribution version always comes from the session, never from the client.
func (s *Server) render(w http.ResponseWriter, r *http.Request) (renderResult, bool) {
	var req struct {
		Answers map[string]any `json:"answers"`
	}

	if !readJSON(w, r, &req) {
		return renderResult{}, false
	}

	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()

	if sess == nil {
		writeError(w, http.StatusBadRequest, ErrNoSession)

		return renderResult{}, false
	}

	if req.Answers == nil {
		req.Answers = map[string]any{}
	}

	req.Answers["version"] = sess.version

	// A yaml field that does not parse is the user's mistake, not the template's: it is reported
	// against the field, and the document is not rendered around broken YAML.
	if errs := CheckYAML(sess.wizard, req.Answers); len(errs) > 0 {
		return renderResult{Errors: errs}, true
	}

	doc, err := Render(sess.tpl, req.Answers)
	if err != nil {
		return renderResult{TemplateError: err.Error()}, true
	}

	errs, err := Validate(sess.schemaPath, doc)
	if err != nil {
		return renderResult{YAML: doc, TemplateError: err.Error()}, true
	}

	if errs == nil {
		errs = []FieldError{}
	}

	return renderResult{YAML: doc, Errors: errs}, true
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	res, ok := s.render(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	res, ok := s.render(w, r)
	if !ok {
		return
	}

	if res.TemplateError != "" || len(res.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, res)

		return
	}

	if _, err := os.Stat(s.outputPath); err == nil {
		writeError(w, http.StatusConflict, fmt.Errorf("%w: %s", ErrOutputExists, s.outputPath))

		return
	}

	if err := os.WriteFile(s.outputPath, []byte(res.YAML), outputFileMode); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("writing %s: %w", s.outputPath, err))

		return
	}

	logrus.Infof("Configuration file written to %s", s.outputPath)
	s.once.Do(func() { close(s.written) })

	writeJSON(w, http.StatusOK, map[string]string{"path": s.outputPath})
}

// handleFile creates one of the files the configuration refers to with {file://…}, so the operator
// does not have to leave the wizard to write an encryption manifest or a certificate.
//
// Deliberately narrow: the path is resolved under the directory the configuration is written to,
// nothing above it can be reached, an existing file is never overwritten unless it is asked for
// explicitly, and the mode is 0600 because most of these files are secrets.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		Content   string `json:"content"`
		Overwrite bool   `json:"overwrite"`
	}

	if !readJSON(w, r, &req) {
		return
	}

	full, err := s.resolveUnderOutput(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	if _, err := os.Stat(full); err == nil && !req.Overwrite {
		writeError(w, http.StatusConflict, fmt.Errorf("%w: %s", ErrFileExists, full))

		return
	}

	if err := os.MkdirAll(filepath.Dir(full), outputDirMode); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("creating the directory: %w", err))

		return
	}

	if err := os.WriteFile(full, []byte(req.Content), outputFileMode); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("writing %s: %w", full, err))

		return
	}

	logrus.Infof("Wrote %s", full)
	writeJSON(w, http.StatusOK, map[string]string{"path": full})
}

// resolveUnderOutput turns the path a {file://…} value carries into an absolute one inside the
// directory of the configuration file, and refuses anything that would land outside it.
func (s *Server) resolveUnderOutput(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: %q", ErrFileOutside, path)
	}

	base, err := filepath.Abs(filepath.Dir(s.outputPath))
	if err != nil {
		return "", fmt.Errorf("resolving the output directory: %w", err)
	}

	full := filepath.Join(base, filepath.Clean(path))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrFileOutside, path)
	}

	return full, nil
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("%w: %w", errDecodeRequest, err))

		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logrus.Errorf("writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// Run serves until the context ends, the user presses ENTER, or a file has been written.
func Run(ctx context.Context, address, port string, s *Server) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	handler, err := s.Handler()
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: address + ":" + port, Handler: handler, ReadHeaderTimeout: readHeaderTimeout}

	errCh := make(chan error, 1)

	go func() { errCh <- srv.ListenAndServe() }()

	go func() {
		if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
			logrus.Debugf("stopped watching stdin: %v", err)

			return
		}

		cancel()
	}()

	logrus.Infof("drydock is listening on http://%s:%s. Press ENTER to stop.", address, port)

	select {
	case <-s.Written():
	case <-ctx.Done():
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("drydock server failed: %w", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down drydock: %w", err)
	}

	logrus.Info("drydock stopped")

	return nil
}
