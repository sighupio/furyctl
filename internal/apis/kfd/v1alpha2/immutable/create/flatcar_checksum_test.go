// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create //nolint:testpackage // calls the unexported downloadFlatcarArtifacts.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	netx "github.com/sighupio/furyctl/pkg/x/net"
)

func TestDownloadFlatcarArtifactsVerifiesChecksums(t *testing.T) {
	t.Parallel()

	// The local source files are the mirror. The file getter copies them.
	srcDir := t.TempDir()
	artifact := func(filename string) flatcarArtifact {
		data := []byte(filename + " bytes")
		path := filepath.Join(srcDir, filename)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}

		sum := sha256.Sum256(data)

		return flatcarArtifact{Filename: filename, URL: path, SHA256: hex.EncodeToString(sum[:])}
	}

	kernel := artifact("flatcar_production_pxe.vmlinuz")
	initrd := artifact("flatcar_production_pxe_image.cpio.gz")
	image := artifact("flatcar_production_image.bin.bz2")
	// The function also downloads the image signature, so the source directory must have this file.
	artifact("flatcar_production_image.bin.bz2.sig")

	badImage := image
	badImage.SHA256 = "deadbeef"

	unpinnedImage := image
	unpinnedImage.SHA256 = ""

	tests := []struct {
		name    string
		image   flatcarArtifact
		wantErr bool
	}{
		{name: "matching digests", image: image},
		{name: "wrong image digest", image: badImage, wantErr: true},
		{name: "no pinned digest", image: unpinnedImage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			downloader := &assetDownloader{
				goGetterClient: netx.NewGoGetterClient(),
				assetsPath:     t.TempDir(),
			}
			release := flatcarRelease{
				Version: "4757.2.1",
				Arch: map[string]flatcarArch{
					"x86-64": {Kernel: kernel, Initrd: initrd, Image: tt.image},
				},
			}

			err := (&Infrastructure{}).downloadFlatcarArtifacts(downloader, release, []string{"x86-64"})

			if !tt.wantErr {
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}

				return
			}

			if !errors.Is(err, ErrChecksumMismatch) {
				t.Fatalf("got %v, want ErrChecksumMismatch", err)
			}

			if !strings.Contains(err.Error(), "image") || !strings.Contains(err.Error(), "x86-64") {
				t.Errorf("error %q does not name the artifact and the architecture", err)
			}
		})
	}
}
