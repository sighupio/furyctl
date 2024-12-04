package utils

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// ResolveOutputDirectory determines the output directory based on user input.
// It defaults to the user's home directory if `outDir` is empty.
func ResolveOutputDirectory(outDir string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Error while getting user home directory: %w", err)
	}

	logrus.Debug("Resolved home directory: ", homeDir)

	if outDir == "" {
		outDir = homeDir
		logrus.Debug("Empty outdir flag. Set outdir to: ", homeDir)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Error while getting current working directory: %w", err)
	}

	logrus.Debug("Resolved current directory: ", currentDir)

	if outDir == "." {
		outDir = currentDir
		logrus.Debug("Set output dir to current directory: ", currentDir)
	}

	return outDir, nil
}
