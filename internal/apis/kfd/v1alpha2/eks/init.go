// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import "github.com/sighupio/furyctl/internal/cluster"

func init() {
	cluster.RegisterCreatorFactory(
		"kfd.sighup.io/v1alpha2",
		"ekscluster",
		func(opts []cluster.CreatorOption) cluster.Creator {
			cc := &ClusterCreator{}
			cc.SetOptions(opts)

			return cc
		},
	)
}
