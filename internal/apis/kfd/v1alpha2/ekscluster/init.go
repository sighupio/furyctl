// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	cluster.RegisterCreatorFactory(
		distribution.APIVersionV1Alpha2,
		distribution.EKSClusterKind,
		cluster.NewCreatorFactory[*ClusterCreator, private.EksclusterKfdV1Alpha2](&ClusterCreator{}),
	)

	cluster.RegisterDeleterFactory(
		distribution.APIVersionV1Alpha2,
		distribution.EKSClusterKind,
		cluster.NewDeleterFactory[*ClusterDeleter, private.EksclusterKfdV1Alpha2](&ClusterDeleter{}),
	)

	cluster.RegisterKubeconfigFactory(
		distribution.APIVersionV1Alpha2,
		distribution.EKSClusterKind,
		cluster.NewKubeconfigFactory[*KubeconfigGetter, private.EksclusterKfdV1Alpha2](&KubeconfigGetter{}),
	)

	cluster.RegisterRenewerFactory(
		distribution.APIVersionV1Alpha2,
		distribution.EKSClusterKind,
		cluster.NewRenewerFactory[*cluster.UnsupportedRenewer, private.EksclusterKfdV1Alpha2](
			&cluster.UnsupportedRenewer{Kind: distribution.EKSClusterKind},
		),
	)
}
