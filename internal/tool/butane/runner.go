// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package butane

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/butane/config"
	"github.com/coreos/butane/config/common"
	"github.com/coreos/vcontext/report"
)

// ErrFatalTranslation is returned when the Butane translation has fatal errors.
var ErrFatalTranslation = errors.New("fatal error in butane config translation")

// Runner provides methods to convert Butane configs to Ignition configs
// using the Butane Go package.
type Runner struct {
	options common.TranslateBytesOptions
}

// NewRunner creates a new Butane runner with default options.
func NewRunner() *Runner {
	return &Runner{
		options: common.TranslateBytesOptions{
			TranslateOptions: common.TranslateOptions{
				FilesDir:                  "",
				NoResourceAutoCompression: false,
				DebugPrintTranslations:    false,
			},
			Pretty: true,
			Raw:    false,
		},
	}
}

// NewRunnerWithOptions creates a new Butane runner with custom options.
func NewRunnerWithOptions(options common.TranslateBytesOptions) *Runner {
	return &Runner{
		options: options,
	}
}

// Convert converts a Butane config (YAML) to an Ignition config (JSON).
// It returns the Ignition JSON bytes and any error encountered.
func (r *Runner) Convert(butaneConfig []byte) ([]byte, error) {
	ignitionJSON, rpt, err := config.TranslateBytes(butaneConfig, r.options)
	if err != nil {
		return nil, translationError(err, rpt)
	}

	if rpt.IsFatal() {
		return nil, fmt.Errorf("%w: %s", ErrFatalTranslation, rpt.String())
	}

	return ignitionJSON, nil
}

// ConvertWithReport converts a Butane config to Ignition and returns the full report.
// This is useful when you want to handle warnings and errors separately.
func (r *Runner) ConvertWithReport(butaneConfig []byte) ([]byte, report.Report, error) {
	ignitionJSON, rpt, err := config.TranslateBytes(butaneConfig, r.options)
	if err != nil {
		return nil, rpt, translationError(err, rpt)
	}

	return ignitionJSON, rpt, nil
}

// translationError wraps a translation failure together with the report, which
// holds the details the user needs to fix the config (path, line, column and
// reason). Butane returns those in the report, not in the error.
func translationError(err error, rpt report.Report) error {
	if details := strings.TrimSpace(rpt.String()); details != "" {
		return fmt.Errorf("error translating butane config: %w:\n%s", err, details)
	}

	return fmt.Errorf("error translating butane config: %w", err)
}

// SetFilesDir sets the directory for embedding local files in butane configs.
func (r *Runner) SetFilesDir(dir string) {
	r.options.FilesDir = dir
}

// SetPretty enables or disables pretty-printing of the output Ignition JSON.
func (r *Runner) SetPretty(pretty bool) {
	r.options.Pretty = pretty
}
