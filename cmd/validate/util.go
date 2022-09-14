package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/distribution"
)

var (
	downloadProtocols = []string{"", "git::", "file::", "http::", "s3::", "gcs::", "mercurial::"}

	errDownloadOptionsExausted = errors.New("downloading options exausted")

	ErrCreatingTempDir     = errors.New("error creating temp dir")
	ErrDownloadingFolder   = errors.New("error downloading folder")
	ErrHasValidationErrors = errors.New("schema has validation errors")
	ErrMergeCompleteConfig = errors.New("error merging complete config")
	ErrMergeDistroConfig   = errors.New("error merging distribution config")
	ErrUnknownOutputFormat = errors.New("unknown output format")
	ErrWriteFile           = errors.New("error writing file")
	ErrYamlMarshalFile     = errors.New("error marshaling yaml file")
	ErrYamlUnmarshalFile   = errors.New("error unmarshaling yaml file")
)

func getSchemaPath(basePath string, conf distribution.FuryctlConfig) string {
	avp := strings.Split(conf.ApiVersion, "/")
	ns := strings.Replace(avp[0], ".sighup.io", "", 1)
	ver := avp[1]
	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind), ns, ver)

	return filepath.Join(basePath, "schemas", filename)
}

func getDefaultPath(basePath string) string {
	return filepath.Join(basePath, "furyctl-defaults.yaml")
}

func downloadDirectory(src string) (string, error) {
	baseDst, err := os.MkdirTemp("", "furyctl-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	dst := filepath.Join(baseDst, "data")

	logrus.Debugf("Downloading '%s' in '%s'", src, dst)

	if err := clientGet(src, dst); err != nil {
		return "", fmt.Errorf("%w '%s': %v", ErrDownloadingFolder, src, err)
	}

	return dst, nil
}

func cleanupTempDir(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logrus.Error(err)
		}
	}
}

// clientGet tries a few different protocols to get the source file or directory.
func clientGet(src, dst string) error {
	protocols := []string{""}
	if !urlHasForcedProtocol(src) {
		protocols = downloadProtocols
	}

	for _, protocol := range protocols {
		fullSrc := fmt.Sprintf("%s%s", protocol, src)

		logrus.Debugf("Trying to download from: %s", fullSrc)

		client := &getter.Client{
			Src:  fullSrc,
			Dst:  dst,
			Mode: getter.ClientModeDir,
		}

		err := client.Get()
		if err == nil {
			return nil
		}

		logrus.Debug(err)
	}

	return errDownloadOptionsExausted
}

// urlHasForcedProtocol checks if the url has a forced protocol as described in hashicorp/go-getter.
func urlHasForcedProtocol(url string) bool {
	for _, dp := range downloadProtocols {
		if dp != "" && strings.HasPrefix(url, dp) {
			return true
		}
	}

	return false
}
