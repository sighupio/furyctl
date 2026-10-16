// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"fmt"
	"path/filepath"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// KFDData reads the distribution's kfd.yaml (distribution and module versions) so that it can be
// exposed to the templates as `.kfd`. It is read as a generic map, with the same keys as the file,
// because the template data is a map: reading it as config.KFD would need a conversion back to a map.
func KFDData(distroPath string) (map[any]any, error) {
	kfdPath := filepath.Join(distroPath, "kfd.yaml")

	kfd, err := yamlx.FromFileV2[map[any]any](kfdPath)
	if err != nil {
		return nil, fmt.Errorf("%s - %w", kfdPath, err)
	}

	return kfd, nil
}
