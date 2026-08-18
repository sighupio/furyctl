// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	cluster.RegisterCreatorFactory(
		distribution.APIVersionV1Alpha2,
		distribution.ImmutableKind,
		cluster.NewCreatorFactory[*ClusterCreator, public.ImmutableKfdV1Alpha2](&ClusterCreator{}),
	)

	cluster.RegisterDeleterFactory(
		distribution.APIVersionV1Alpha2,
		distribution.ImmutableKind,
		cluster.NewDeleterFactory[*ClusterDeleter, public.ImmutableKfdV1Alpha2](&ClusterDeleter{}),
	)

	cluster.RegisterKubeconfigFactory(
		distribution.APIVersionV1Alpha2,
		distribution.ImmutableKind,
		cluster.NewKubeconfigFactory[*KubeconfigGetter, public.ImmutableKfdV1Alpha2](&KubeconfigGetter{}),
	)

	cluster.RegisterCertificatesRenewerFactory(
		distribution.APIVersionV1Alpha2,
		distribution.ImmutableKind,
		cluster.NewCertificatesRenewerFactory[*CertificatesRenewer, public.ImmutableKfdV1Alpha2](&CertificatesRenewer{}),
	)
}
