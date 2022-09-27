package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/osx"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	"github.com/sighupio/furyctl/internal/yaml"
)

// Validate the furyctl.yaml file
func Validate(path, repoPath string) error {
	defaultsPath := distribution.GetDefaultsPath(repoPath)

	defaultedFuryctlConfPath, err := mergeWithDefaults(path, defaultsPath)
	if err != nil {
		return err
	}

	defer osx.CleanupTempDir(filepath.Base(defaultedFuryctlConfPath))

	miniConf, err := loadFromFile(path)
	if err != nil {
		return err
	}

	schemaPath, err := distribution.GetSchemaPath(repoPath, miniConf)
	if err != nil {
		return err
	}

	schema, err := santhosh.LoadSchema(schemaPath)
	if err != nil {
		return err
	}

	conf, err := yaml.FromFileV3[any](defaultedFuryctlConfPath)
	if err != nil {
		return err
	}

	return schema.ValidateInterface(conf)
}

func loadFromFile(path string) (config.Furyctl, error) {
	conf, err := yaml.FromFileV3[config.Furyctl](path)
	if err != nil {
		return config.Furyctl{}, err
	}

	if err := config.NewValidator().Struct(conf); err != nil {
		return config.Furyctl{}, err
	}

	return conf, err
}

func mergeWithDefaults(furyctlConfPath, defaultsConfPath string) (string, error) {
	defaultsFile, err := yaml.FromFileV2[map[any]any](defaultsConfPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrYamlUnmarshalFile, err)
	}

	furyctlFile, err := yaml.FromFileV2[map[any]any](furyctlConfPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrYamlUnmarshalFile, err)
	}

	defaultsModel := merge.NewDefaultModel(defaultsFile, ".data")
	distributionModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")

	distroMerger := merge.NewMerger(defaultsModel, distributionModel)

	defaultedDistribution, err := distroMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrMergeDistroConfig, err)
	}

	furyctlModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")
	defaultedDistributionModel := merge.NewDefaultModel(defaultedDistribution, ".data")

	furyctlMerger := merge.NewMerger(furyctlModel, defaultedDistributionModel)

	defaultedFuryctl, err := furyctlMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrMergeCompleteConfig, err)
	}

	outYaml, err := yaml.MarshalV2(defaultedFuryctl)
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrYamlMarshalFile, err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-defaulted-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrCreatingTempDir, err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")
	if err := os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return "", fmt.Errorf("%w: %v", distribution.ErrWriteFile, err)
	}

	return confPath, nil
}
