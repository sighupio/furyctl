// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	cluster.RegisterCreatorFactory(
		"kfd.sighup.io/v1alpha2",
		"EKSCluster",
		cluster.NewCreatorFactory[*ClusterCreator, schema.EksclusterKfdV1Alpha2](&ClusterCreator{}),
	)

	cluster.RegisterDeleterFactory(
		"kfd.sighup.io/v1alpha2",
		"EKSCluster",
		cluster.NewDeleterFactory[*ClusterDeleter, schema.EksclusterKfdV1Alpha2](&ClusterDeleter{}),
	)
}
