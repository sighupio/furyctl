package diffs

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/state"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

func NewStateChecker(stateStore state.Storer, furyctlPath string) (Checker, error) {
	var diffChecker Checker

	storedCfg := map[string]any{}

	storedCfgStr, err := stateStore.GetConfig()
	if err != nil {
		return diffChecker, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	if err := yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return diffChecker, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](furyctlPath)
	if err != nil {
		return diffChecker, fmt.Errorf("error while reading config file: %w", err)
	}

	return NewBaseChecker(storedCfg, newCfg), nil
}
