// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

var ErrClusterDeletionNotImplemented = errors.New("cluster deletion not implemented for Immutable kind")

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
}

func (c *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}
}

func (c *ClusterDeleter) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.DeleterPropertyConfigPath:
		if s, ok := value.(string); ok {
			c.paths.ConfigPath = s
		}

	case cluster.DeleterPropertyDistroPath:
		if s, ok := value.(string); ok {
			c.paths.DistroPath = s
		}

	case cluster.DeleterPropertyWorkDir:
		if s, ok := value.(string); ok {
			c.paths.WorkDir = s
		}

	case cluster.DeleterPropertyBinPath:
		if s, ok := value.(string); ok {
			c.paths.BinPath = s
		}

	case cluster.DeleterPropertyFuryctlConf:
		if s, ok := value.(public.ImmutableKfdV1Alpha2); ok {
			c.furyctlConf = s
		}

	case cluster.DeleterPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			c.kfdManifest = s
		}

	default:
		logrus.Debugf("ignoring unknown property %q", lcName)
	}
}

func (*ClusterDeleter) Delete() error {
	return ErrClusterDeletionNotImplemented
}
