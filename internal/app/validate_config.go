package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/app/validate"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	"github.com/sighupio/furyctl/internal/yaml"
)

type ValidateConfigRequest struct {
	FuryctlBinVersion string
	DistroLocation    string
	FuryctlConfPath   string
	Debug             bool
}

type ValidateConfigResponse struct {
	Error    error
	RepoPath string
}

func (v ValidateConfigResponse) HasErrors() bool {
	return v.Error != nil
}

func NewValidateConfig() *ValidateConfig {
	return &ValidateConfig{}
}

type ValidateConfig struct{}

func (h *ValidateConfig) Execute(req ValidateConfigRequest) (ValidateConfigResponse, error) {
	res, err := validate.DownloadDistro(req.FuryctlBinVersion, req.DistroLocation, req.FuryctlConfPath, req.Debug)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	schemaPath, err := validate.GetSchemaPath(res.RepoPath, res.MinimalConf)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	defaultPath := validate.GetDefaultPath(res.RepoPath)

	defaultedFuryctlConfPath, err := h.mergeConfigAndDefaults(req.FuryctlConfPath, defaultPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}
	if !req.Debug {
		defer validate.CleanupTempDir(filepath.Base(defaultedFuryctlConfPath))
	}

	schema, err := santhosh.LoadSchema(schemaPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	conf, err := yaml.FromFileV3[any](defaultedFuryctlConfPath)
	if err != nil {
		return ValidateConfigResponse{}, err
	}

	if err := schema.ValidateInterface(conf); err != nil {
		return ValidateConfigResponse{
			RepoPath: res.RepoPath,
			Error:    err,
		}, nil
	}

	return ValidateConfigResponse{}, nil
}

func (h *ValidateConfig) mergeConfigAndDefaults(furyctlFilePath, defaultsFilePath string) (string, error) {
	defaultsFile, err := yaml.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrYamlUnmarshalFile, err)
	}

	furyctlFile, err := yaml.FromFileV2[map[any]any](furyctlFilePath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrYamlUnmarshalFile, err)
	}

	defaultsModel := merge.NewDefaultModel(defaultsFile, ".data")
	distributionModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")

	distroMerger := merge.NewMerger(defaultsModel, distributionModel)

	defaultedDistribution, err := distroMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrMergeDistroConfig, err)
	}

	furyctlModel := merge.NewDefaultModel(furyctlFile, ".spec.distribution")
	defaultedDistributionModel := merge.NewDefaultModel(defaultedDistribution, ".data")

	furyctlMerger := merge.NewMerger(furyctlModel, defaultedDistributionModel)

	defaultedFuryctl, err := furyctlMerger.Merge()
	if err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrMergeCompleteConfig, err)
	}

	outYaml, err := yaml.MarshalV2(defaultedFuryctl)
	if err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrYamlMarshalFile, err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-defaulted-")
	if err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrCreatingTempDir, err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")
	if err := os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return "", fmt.Errorf("%w: %v", validate.ErrWriteFile, err)
	}

	return confPath, nil
}
