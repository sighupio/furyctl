// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

type Kubernetes struct {
	criticalTFResourceTypes []string
}

func NewKubernetes() *Kubernetes {
	return &Kubernetes{
		criticalTFResourceTypes: []string{"aws_eks_cluster"},
	}
}

func (k *Kubernetes) GetCriticalTFResourceTypes() []string {
	return k.criticalTFResourceTypes
}
