package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

func ValidateChecksum(tl Tool, checksums map[string]string) error {
	osArch := fmt.Sprintf("%s/%s", tl.OS(), tl.Arch())

	checksum, exist := checksums[osArch]
	if !exist {
		return fmt.Errorf("unable to get checksum for %s", osArch)
	}

	fileBytes, err := os.ReadFile(tl.CmdPath())
	if err != nil {
		return err
	}

	fileChecksumBytes := sha256.Sum256(fileBytes)
	fileChecksum := hex.EncodeToString(fileChecksumBytes[:])

	if checksum != fileChecksum {
		return fmt.Errorf("checksum mismatch for %s", osArch)
	}

	return nil
}
