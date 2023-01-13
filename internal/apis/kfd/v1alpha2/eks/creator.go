// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrUnsupportedPhase = errors.New("unsupported phase")

type ClusterCreator struct {
	paths          cluster.CreatorPaths
	furyctlConf    schema.EksclusterKfdV1Alpha2
	kfdManifest    config.KFD
	phase          string
	vpnAutoConnect bool
}

func (v *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		v.SetProperty(prop.Name, prop.Value)
	}
}

func (v *ClusterCreator) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.CreatorPropertyConfigPath:
		if s, ok := value.(string); ok {
			v.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(schema.EksclusterKfdV1Alpha2); ok {
			v.furyctlConf = s
		}

	case cluster.CreatorPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			v.kfdManifest = s
		}

	case cluster.CreatorPropertyPhase:
		if s, ok := value.(string); ok {
			v.phase = s
		}

	case cluster.CreatorPropertyVpnAutoConnect:
		if b, ok := value.(bool); ok {
			v.vpnAutoConnect = b
		}

	case cluster.CreatorPropertyDistroPath:
		if s, ok := value.(string); ok {
			v.paths.DistroPath = s
		}

	case cluster.CreatorPropertyWorkDir:
		if s, ok := value.(string); ok {
			v.paths.WorkDir = s
		}

	case cluster.CreatorPropertyBinPath:
		if s, ok := value.(string); ok {
			v.paths.BinPath = s
		}

	case cluster.CreatorPropertyKubeconfig:
		if s, ok := value.(string); ok {
			v.paths.Kubeconfig = s
		}
	}
}

func (v *ClusterCreator) Create(dryRun bool, skipPhase string) error {
	infra, err := create.NewInfrastructure(v.furyctlConf, v.kfdManifest, v.paths, dryRun)
	if err != nil {
		return fmt.Errorf("error while initiating infrastructure phase: %w", err)
	}

	kube, err := create.NewKubernetes(
		v.furyctlConf,
		v.kfdManifest,
		infra.OutputsPath,
		v.paths,
		dryRun,
	)
	if err != nil {
		return fmt.Errorf("error while initiating kubernetes phase: %w", err)
	}

	distro, err := create.NewDistribution(
		v.paths,
		v.furyctlConf,
		v.kfdManifest,
		infra.OutputsPath,
		dryRun,
	)
	if err != nil {
		return fmt.Errorf("error while initiating distribution phase: %w", err)
	}

	infraOpts := []cluster.OperationPhaseOption{
		{Name: cluster.OperationPhaseOptionVPNAutoConnect, Value: v.vpnAutoConnect},
	}

	switch v.phase {
	case cluster.OperationPhaseInfrastructure:
		if err = infra.Exec(infraOpts); err != nil {
			return fmt.Errorf("error while executing infrastructure phase: %w", err)
		}

		return nil

	case cluster.OperationPhaseKubernetes:
		if err = kube.Exec(); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		if !dryRun {
			if err := v.storeClusterConfig(); err != nil {
				return fmt.Errorf("error while storing cluster config: %w", err)
			}
		}

		return nil

	case cluster.OperationPhaseDistribution:
		if err = distro.Exec(); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}

		if !dryRun {
			if err := v.storeClusterConfig(); err != nil {
				return fmt.Errorf("error while storing cluster config: %w", err)
			}
		}

		return nil

	case cluster.OperationPhaseAll:
		if v.furyctlConf.Spec.Infrastructure != nil &&
			(skipPhase == "" || skipPhase == cluster.OperationPhaseDistribution) {
			if err := infra.Exec(infraOpts); err != nil {
				return fmt.Errorf("error while executing infrastructure phase: %w", err)
			}
		}

		if skipPhase != cluster.OperationPhaseKubernetes {
			if err := kube.Exec(); err != nil {
				return fmt.Errorf("error while executing kubernetes phase: %w", err)
			}

			if err := v.storeClusterConfig(); err != nil {
				return fmt.Errorf("error while storing cluster config: %w", err)
			}
		}

		if skipPhase != cluster.OperationPhaseDistribution {
			if err = distro.Exec(); err != nil {
				return fmt.Errorf("error while executing distribution phase: %w", err)
			}

			if err := v.storeClusterConfig(); err != nil {
				return fmt.Errorf("error while storing cluster config: %w", err)
			}
		}

		return nil

	default:
		return ErrUnsupportedPhase
	}
}

func (v *ClusterCreator) storeClusterConfig() error {
	x, err := os.ReadFile(v.paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret := kubex.CreateSecret(x, "furyctl-config", "kube-system")

	sYaml, err := yamlx.MarshalV2(secret)
	if err != nil {
		return fmt.Errorf("error while marshaling secret: %w", err)
	}

	if err := iox.WriteFile(path.Join(v.paths.WorkDir, "secret.yaml"), sYaml); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(path.Join(v.paths.WorkDir, "secret.yaml"))

	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl:    path.Join(v.paths.BinPath, "kubectl"),
		WorkDir:    v.paths.WorkDir,
		Kubeconfig: path.Join(v.paths.WorkDir, "secret.yaml"),
	}, true, true)

	logrus.Info("Storing cluster config...")

	if err := runner.Apply(secret); err != nil {
		return fmt.Errorf("error while storing cluster config: %w", err)
	}

	return nil
}
