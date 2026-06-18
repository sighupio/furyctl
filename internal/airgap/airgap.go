// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package airgap wires the --airgap-bundle flow: it extracts a self-contained bundle (produced by
// `furyctl download air-gapped-bundle`) into the working dir and rewires the running command to
// consume it offline, so a single flag replaces the manual
// `--skip-deps-download --distro-location ./distro` dance.
package airgap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

const (
	// Records the checksum of the last successfully extracted bundle, so re-running a command with
	// the same bundle skips a (potentially large) re-extraction.
	markerFile = ".airgap-bundle.sha256"

	// DistroSubdir is the folder, inside the bundle, holding the distribution manifests used as
	// --distro-location.
	DistroSubdir = "distro"
)

var ErrBundleNotFound = errors.New("air-gapped bundle not found")

// RegisterFlags adds the --airgap-bundle and --force-extract flags to a command that can consume a
// bundle. Use together with MaybePrepare at the start of the command's RunE.
func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().String(
		"airgap-bundle",
		"",
		"Path to an air-gapped bundle (.tar.gz) produced by 'furyctl download air-gapped-bundle'. "+
			"When set, furyctl extracts it into the working directory and runs fully offline "+
			"(implies --skip-deps-download and --distro-location)",
	)

	cmd.Flags().Bool(
		"force-extract",
		false,
		"Force re-extraction of the --airgap-bundle even when it was already extracted",
	)
}

// MaybePrepare extracts the bundle referenced by --airgap-bundle (if any) into the outdir and rewires
// viper so the command runs fully offline. It is a no-op when --airgap-bundle is unset. Extraction is
// idempotent: it is skipped when the bundle checksum matches the recorded marker, unless
// --force-extract is set.
func MaybePrepare() error {
	bundle := viper.GetString("airgap-bundle")
	if bundle == "" {
		return nil
	}

	outDir := viper.GetString("outdir")
	force := viper.GetBool("force-extract")

	distroLocation, err := prepare(bundle, outDir, force)
	if err != nil {
		return err
	}

	// A single flag replaces the manual offline wiring.
	viper.Set("skip-deps-download", true)
	viper.Set("distro-location", distroLocation)

	return nil
}

//nolint:revive // force is an explicit user choice (--force-extract), not an internal mode toggle.
func prepare(bundle, outDir string, force bool) (string, error) {
	bundle, err := filepath.Abs(bundle)
	if err != nil {
		return "", fmt.Errorf("error resolving bundle path: %w", err)
	}

	if _, err := os.Stat(bundle); err != nil {
		return "", fmt.Errorf("%w: %s", ErrBundleNotFound, bundle)
	}

	sum, err := iox.Sha256File(bundle)
	if err != nil {
		return "", fmt.Errorf("error checksumming bundle: %w", err)
	}

	marker := filepath.Join(outDir, ".furyctl", markerFile)
	distroLocation := filepath.Join(outDir, DistroSubdir)

	if !force && bundleAlreadyExtracted(marker, sum) {
		logrus.Infof("Air-gapped bundle already extracted, reusing it (use --force-extract to re-extract)")

		return distroLocation, nil
	}

	logrus.Infof("Extracting air-gapped bundle %s ...", bundle)

	if err := iox.ExtractTarGz(bundle, outDir); err != nil {
		return "", fmt.Errorf("error extracting air-gapped bundle: %w", err)
	}

	if err := writeMarker(marker, sum); err != nil {
		return "", err
	}

	return distroLocation, nil
}

func bundleAlreadyExtracted(marker, sum string) bool {
	recorded, err := os.ReadFile(marker)
	if err != nil {
		return false
	}

	return string(recorded) == sum
}

func writeMarker(marker, sum string) error {
	if err := os.MkdirAll(filepath.Dir(marker), iox.FullPermAccess); err != nil {
		return fmt.Errorf("error creating marker dir: %w", err)
	}

	if err := os.WriteFile(marker, []byte(sum), iox.RWPermAccess); err != nil {
		return fmt.Errorf("error writing bundle marker: %w", err)
	}

	return nil
}
