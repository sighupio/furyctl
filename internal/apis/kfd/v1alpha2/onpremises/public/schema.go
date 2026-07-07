// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package public contains furyctl's curated, hand-maintained view of the
// OnPremises furyctl.yaml (apiVersion kfd.sighup.io/v1alpha2).
//
// It models ONLY the fields furyctl actually reads. The full furyctl.yaml is
// validated at runtime against the JSON schema shipped by the distribution
// (see internal/config.Validate); fields not modeled here are still validated
// there and still reach the templates via the raw YAML, so omitting them is
// safe. Keep this struct readable: it should read like a furyctl.yaml skeleton.
package public

// Kind is the furyctl.yaml kind discriminator (OnPremises).
type Kind string

// OnpremisesKfdV1Alpha2 is furyctl's read-view of an OnPremises config.
type OnpremisesKfdV1Alpha2 struct {
	Kind Kind `yaml:"kind"`
	Spec Spec `yaml:"spec"`
}

type Spec struct {
	Kubernetes Kubernetes `yaml:"kubernetes"`
}

type Kubernetes struct {
	Advanced *Advanced `yaml:"advanced,omitempty"`
}

type Advanced struct {
	Users *Users `yaml:"users,omitempty"`
}

type Users struct {
	// Names of the OS users furyctl provisions on the nodes.
	Names []string `yaml:"names,omitempty"`
}
