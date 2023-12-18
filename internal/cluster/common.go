// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"errors"
	"fmt"
	"os"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
)

var ErrCannotSetEnvVar = errors.New("cannot set env var")

func resetKubeconfigEnv(kfdManifest config.KFD) error {
	if distribution.HasFeature(kfdManifest, distribution.FeatureKubeconfigInSchema) {
		if err := os.Setenv("KUBECONFIG", "willingly-invalid-kubeconfig-path-to-avoid-accidental-usage"); err != nil {
			return fmt.Errorf("%w: %w", ErrCannotSetEnvVar, err)
		}
	}

	return nil
}
