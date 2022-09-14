package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	yaml2 "github.com/sighupio/furyctl/internal/yaml"
)

const defaultBaseUrl = "https://git@github.com/sighupio/fury-distribution//%s?ref=feature/create-draft-of-the-furyctl-yaml-json-schema"

var (
	downloadProtocols = []string{"", "git::", "file::", "http::", "s3::", "gcs::", "mercurial::"}

	ErrHasValidationErrors = errors.New("schema has validation errors")
	ErrUnknownOutputFormat = errors.New("unknown output format")
	ErrDownloadingSchemas  = errors.New("error downloading schemas")
	ErrCreatingTempDir     = errors.New("error creating temp dir")
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
		return "", fmt.Errorf("%s: %w", ErrCreatingTempDir, err)
	}

	logrus.Debugf("Downloading '%s' from '%s' in '%s'", name, src, dir)

	return dir, clientGet(src, dir)
}

func MergeConfigAndDefaults(furyctlFilePath string, defaultsFilePath string) (string, error) {
	defaultsFile, err := yaml2.FromFile[map[any]any](defaultsFilePath)
	if err != nil {
		return "", err
	}

	furyctlFile, err := yaml2.FromFile[map[any]any](furyctlFilePath)
	if err != nil {
		return "", err
	}

	defaultsModel := merge.NewDefaultModel(defaultsFile, ".data")
	distributionModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")

	merger := merge.NewMerger(defaultsModel, distributionModel)

	defaultedDistribution, err := merger.Merge()
	if err != nil {
		return "", err
	}

	furyctlModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")
	defaultedDistributionModel := merge.NewDefaultModel(defaultedDistribution, ".data")

	merger2 := merge.NewMerger(furyctlModel, defaultedDistributionModel)

	defaultedFuryctl, err := merger2.Merge()
	if err != nil {
		return "", err
	}

	outYaml, err := yaml.Marshal(defaultedFuryctl)
	if err != nil {
		return "", err
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-defaulted-")
	if err != nil {
		return "", err
	}

	confPath := filepath.Join(outDirPath, "config.yaml")
	if err := os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return "", err
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

func PrintResults(err error, conf any, configFile string) {
	ptrPaths := santhosh.GetPtrPaths(err)

	fmt.Printf("CONFIG FILE %s\n", configFile)

	for _, path := range ptrPaths {
		value, serr := santhosh.GetValueAtPath(conf, path)
		if serr != nil {
			log.Fatal(serr)
		}

		fmt.Printf(
			"path '%s' contains an invalid configuration value: %+v\n",
			santhosh.JoinPtrPath(path),
			value,
		)
	}

	fmt.Println(err)
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

	return ErrDownloadingSchemas
}

func urlHasForcedProtocol(url string) bool {
	for _, dp := range downloadProtocols {
		if strings.HasPrefix(url, dp) {
			return true
		}
	}

	return false
}
