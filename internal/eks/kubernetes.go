// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

type Kubernetes struct {
	criticalTFResources []string
}

func NewKubernetes() *Kubernetes {
	return &Kubernetes{
		criticalTFResources: []string{"aws_eks_cluster"},
	}
}

func (k *Kubernetes) GetCriticalTFResources() []string {
	return k.criticalTFResources
}
