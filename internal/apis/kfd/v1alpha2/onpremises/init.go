// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	cluster.RegisterCreatorFactory(
		"kfd.sighup.io/v1alpha2",
		"OnPremises",
		cluster.NewCreatorFactory[*ClusterCreator, public.OnpremisesKfdV1Alpha2](&ClusterCreator{}),
	)

	cluster.RegisterDeleterFactory(
		"kfd.sighup.io/v1alpha2",
		"OnPremises",
		cluster.NewDeleterFactory[*ClusterDeleter, public.OnpremisesKfdV1Alpha2](&ClusterDeleter{}),
	)

	cluster.RegisterSchemaSettings(
		"kfd.sighup.io/v1alpha2",
		"OnPremises",
		NewSchemaSettings(),
	)
}
