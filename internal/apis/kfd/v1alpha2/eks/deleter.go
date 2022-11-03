// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"
	"strings"

	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/delete"
	"github.com/sighupio/furyctl/internal/cluster"
)

type ClusterDeleter struct {
	force bool
}

func (v *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		v.SetProperty(prop.Name, prop.Value)
	}
}

func (v *ClusterDeleter) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	if lcName == cluster.DeleterPropertyForce {
		if b, ok := value.(bool); ok {
			v.force = b
		}
	}
}

func (*ClusterDeleter) Delete() error {
	distro, err := del.NewDistribution()
	if err != nil {
		return fmt.Errorf("error while creating distribution phase: %w", err)
	}

	kube, err := del.NewKubernetes()
	if err != nil {
		return fmt.Errorf("error while creating kubernetes phase: %w", err)
	}

	infra, err := del.NewInfrastructure()
	if err != nil {
		return fmt.Errorf("error while creating infrastructure phase: %w", err)
	}

	if err := distro.Exec(); err != nil {
		return fmt.Errorf("error while deleting distribution phase: %w", err)
	}

	if err := kube.Exec(); err != nil {
		return fmt.Errorf("error while deleting kubernetes phase: %w", err)
	}

	if err := infra.Exec(); err != nil {
		return fmt.Errorf("error while deleting infrastructure phase: %w", err)
	}

	return nil
}
