package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/merge"
	yaml2 "github.com/sighupio/furyctl/internal/yaml"
)

const defaultBaseUrl = "https://git@github.com/sighupio/fury-distribution//%s?ref=feature/create-draft-of-the-furyctl-yaml-json-schema"

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

type FuryctlConfig struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		DistributionVersion string `yaml:"distributionVersion"`
	} `yaml:"spec"`
}

func GetSchemaPath(basePath string, conf FuryctlConfig) string {
	avp := strings.Split(conf.ApiVersion, "/")
	ns := strings.Replace(avp[0], ".sighup.io", "", 1)
	ver := avp[1]
	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind), ns, ver)

	return filepath.Join(basePath, conf.Spec.DistributionVersion, filename)
}

func GetDefaultPath(basePath string, conf FuryctlConfig) string {
	return filepath.Join(basePath, conf.Spec.DistributionVersion, "furyctl-defaults.yaml")
}

func DownloadFolder(distroLocation string, name string) (string, error) {
	src := fmt.Sprintf(defaultBaseUrl, name)
	if distroLocation != "" {
		src = distroLocation
	}

	dir, err := os.MkdirTemp("", fmt.Sprintf("furyctl-%s-", name))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	logrus.Debugf("Downloading '%s' from '%s' in '%s'", name, src, dir)

	if err := clientGet(src, dir); err != nil {
		return "", fmt.Errorf("%w '%s': %v", ErrDownloadingFolder, src, err)
	}

	return dir, nil
}

func MergeConfigAndDefaults(furyctlFilePath string, defaultsFilePath string) (string, error) {
	defaultsFile, err := yaml2.FromFile[map[any]any](defaultsFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrYamlUnmarshalFile, err)
	}

	furyctlFile, err := yaml2.FromFile[map[any]any](furyctlFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrYamlUnmarshalFile, err)
	}

	defaultsModel := merge.NewDefaultModel(defaultsFile, ".data")
	distributionModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")

	distroMerger := merge.NewMerger(defaultsModel, distributionModel)

	defaultedDistribution, err := distroMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMergeDistroConfig, err)
	}

	furyctlModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")
	defaultedDistributionModel := merge.NewDefaultModel(defaultedDistribution, ".data")

	furyctlMerger := merge.NewMerger(furyctlModel, defaultedDistributionModel)

	defaultedFuryctl, err := furyctlMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMergeCompleteConfig, err)
	}

	outYaml, err := yaml.Marshal(defaultedFuryctl)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrYamlMarshalFile, err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-defaulted-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCreatingTempDir, err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")
	if err := os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return "", fmt.Errorf("%w: %v", ErrWriteFile, err)
	}

	return confPath, nil
}

func PrintSummary(hasErrors bool) {
	if hasErrors {
		fmt.Println("Validation failed")
	} else {
		fmt.Println("Validation succeeded")
	}
}

func PrintResults(err error, configFile string) {
	fmt.Printf("CONFIG FILE %s\n", configFile)

	fmt.Println(err)
}

func CleanupTempDir(dir string) {
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

		if err := client.Get(); err == nil {
			logrus.Debug("Download successful")

			return nil
		}
	}

	return errDownloadOptionsExausted
}

// urlHasForcedProtocol checks if the url has a forced protocol as described in hashicorp/go-getter.
func urlHasForcedProtocol(url string) bool {
	for _, dp := range downloadProtocols {
		if strings.HasPrefix(url, dp) {
			return true
		}
	}

	return false
}
