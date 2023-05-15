// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

var (
	ErrChecksumMismatch    = errors.New("checksum mismatch")
	ErrUnableToGetChecksum = errors.New("unable to get checksum")
)

func ValidateChecksum(tl Tool, checksums map[string]string) error {
	osArch := fmt.Sprintf("%s/%s", tl.OS(), tl.Arch())

	checksum, exist := checksums[osArch]
	if !exist {
		return fmt.Errorf("%w for %s", ErrUnableToGetChecksum, osArch)
	}

	fileBytes, err := os.ReadFile(tl.CmdPath())
	if err != nil {
		return fmt.Errorf("unable to read file: %w", err)
	}

	fileChecksumBytes := sha256.Sum256(fileBytes)
	fileChecksum := hex.EncodeToString(fileChecksumBytes[:])

	if checksum != fileChecksum {
		return fmt.Errorf("%w for %s", ErrChecksumMismatch, osArch)
	}

	return nil
}
