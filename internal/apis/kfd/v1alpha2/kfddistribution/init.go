// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kfddistribution

import (
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	cluster.RegisterCreatorFactory(
		"kfd.sighup.io/v1alpha2",
		"KFDDistribution",
		cluster.NewCreatorFactory[*ClusterCreator, public.KfddistributionKfdV1Alpha2](&ClusterCreator{}),
	)

	cluster.RegisterDeleterFactory(
		"kfd.sighup.io/v1alpha2",
		"KFDDistribution",
		cluster.NewDeleterFactory[*ClusterDeleter, public.KfddistributionKfdV1Alpha2](&ClusterDeleter{}),
	)

	cluster.RegisterKubeconfigFactory(
		"kfd.sighup.io/v1alpha2",
		"KFDDistribution",
		cluster.NewKubeconfigFactory[*KubeconfigGetter, public.KfddistributionKfdV1Alpha2](&KubeconfigGetter{}),
	)

	cluster.RegisterCertificatesRenewerFactory(
		"kfd.sighup.io/v1alpha2",
		"KFDDistribution",
		cluster.NewCertificatesRenewerFactory[*CertificatesRenewer, public.KfddistributionKfdV1Alpha2](
			&CertificatesRenewer{},
		),
	)
}
