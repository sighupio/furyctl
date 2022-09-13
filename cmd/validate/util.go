package validate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	log "github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

const defaultSchemaBaseUrl = "git::https://git@github.com/sighupio/fury-distribution//schemas?ref=feature/create-draft-of-the-furyctl-yaml-json-schema"

var (
	ErrHasValidationErrors = errors.New("schema has validation errors")
	ErrUnknownOutputFormat = errors.New("unknown output format")
)

type FuryctlConfig struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		DistributionVersion string `yaml:"distributionVersion"`
	} `yaml:"spec"`
}

func GetSchemaPath(basePath string, conf FuryctlConfig) string {
	parts := strings.Split(conf.ApiVersion, "/")
	foo := strings.Replace(parts[0], ".sighup.io", "", 1)
	bar := parts[1]
	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind), foo, bar)

	return filepath.Join(basePath, conf.Spec.DistributionVersion, filename)
}

func ParseArgs(args []string) (string, error) {
	basePath := cmdutil.GetWd()

	furyctlFile := filepath.Join(basePath, "furyctl.yaml")

	if len(args) == 1 {
		furyctlFile = args[0]
	}

	if len(args) > 1 {
		return "", fmt.Errorf("%v, only 1 expected", cmdutil.ErrTooManyArguments)
	}

	return furyctlFile, nil
}

func DownloadSchemas(distroLocation string) (string, error) {
	src := defaultSchemaBaseUrl
	if distroLocation != "" {
		src = distroLocation
	}

	dst := "/tmp/fury-distribution/schemas"

	client := &getter.Client{
		Src:  src,
		Dst:  dst,
		Mode: getter.ClientModeDir,
	}

	return dst, client.Get()
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
