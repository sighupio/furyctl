// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"strings"

	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
)

var crDiffers = make(map[string]map[string]DifferFactory) //nolint:gochecknoglobals, lll // This patterns requires crDiffer

type Differ interface {
	DiffInfrastructurePhase() ([]byte, error)
	DiffKubernetesPhase() ([]byte, error)
	DiffDistributionPhase() ([]byte, error)
}

type DifferFactory func(
	shellRunner *shell.Runner,
	kubectlRunner *kubectl.Runner,
) Differ

func RegisterDifferFactory(apiVersion, kind string, factory DifferFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := crDiffers[lcAPIVersion]; !ok {
		crDiffers[lcAPIVersion] = make(map[string]DifferFactory)
	}

	crDiffers[lcAPIVersion][lcKind] = factory
}

func NewDiffer(apiVersion, kind string, shellRunner *shell.Runner, kubectlRunner *kubectl.Runner) Differ {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if factory, ok := crDiffers[lcAPIVersion][lcKind]; ok {
		return factory(shellRunner, kubectlRunner)
	}

	return nil
}
