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
	switch strings.ToLower(name) {
	case cluster.DeleterPropertyConfigPath:
		cluster.SetPropertyValue(value, &c.paths.ConfigPath)
	case cluster.DeleterPropertyDistroPath:
		cluster.SetPropertyValue(value, &c.paths.DistroPath)
	case cluster.DeleterPropertyWorkDir:
		cluster.SetPropertyValue(value, &c.paths.WorkDir)
	case cluster.DeleterPropertyBinPath:
		cluster.SetPropertyValue(value, &c.paths.BinPath)
	case cluster.DeleterPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &c.furyctlConf)
	case cluster.DeleterPropertyKfdManifest:
		cluster.SetPropertyValue(value, &c.kfdManifest)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (*ClusterDeleter) Delete() error {
	return ErrClusterDeletionNotImplemented
}
