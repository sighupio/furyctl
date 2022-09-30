package eks

import "github.com/sighupio/furyctl/internal/cluster"

func init() {
	cluster.RegisterCreatorFactory(
		"kfd.sighup.io/v1alpha2",
		"ekscluster",
		func(opts []cluster.CreatorOption[any]) cluster.Creator {
			cc := &ClusterCreator{}
			cc.SetOptions(opts)

			return cc
		},
	)
}
