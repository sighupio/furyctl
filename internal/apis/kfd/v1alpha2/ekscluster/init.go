// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/cluster"
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	cluster.RegisterCreatorFactory(
		"kfd.sighup.io/v1alpha2",
		"EKSCluster",
		cluster.NewCreatorFactory[*ClusterCreator, private.EksclusterKfdV1Alpha2](&ClusterCreator{}),
	)

	cluster.RegisterDeleterFactory(
		"kfd.sighup.io/v1alpha2",
		"EKSCluster",
		cluster.NewDeleterFactory[*ClusterDeleter, private.EksclusterKfdV1Alpha2](&ClusterDeleter{}),
	)

	cluster.RegisterSchemaSettings(
		"kfd.sighup.io/v1alpha2",
		"EKSCluster",
		NewSchemaSettings(),
	)

	cluster.RegisterDifferFactory(
		"kfd.sighup.io/v1alpha2",
		"EKSCluster",
		NewClusterDiffer,
	)
}
