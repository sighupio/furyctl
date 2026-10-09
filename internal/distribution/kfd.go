// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"fmt"
	"path/filepath"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// KFDTemplateData reads the distribution's kfd.yaml (distribution and module versions) so that it
// can be exposed to the templates as `.kfd`. It is read as a generic map rather than config.KFD so
// that modules not yet modeled in furyctl are not silently dropped.
func KFDTemplateData(distroPath string) (map[any]any, error) {
	kfdPath := filepath.Join(distroPath, "kfd.yaml")

	kfd, err := yamlx.FromFileV2[map[any]any](kfdPath)
	if err != nil {
		return nil, fmt.Errorf("%s - %w", kfdPath, err)
	}

	return kfd, nil
}
