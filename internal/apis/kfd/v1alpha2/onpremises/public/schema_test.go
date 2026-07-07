// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package public_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
)

// TestAntiDrift decodes a furyctl.yaml exercising the fields furyctl reads from
// an OnPremises config and asserts they decode (guards yaml-tag drift from the
// distribution schema).
func TestAntiDrift(t *testing.T) {
	t.Parallel()

	const sample = `
kind: OnPremises
spec:
  kubernetes:
    advanced:
      users:
        names:
          - alice
          - bob
`

	var c public.OnpremisesKfdV1Alpha2
	if err := yaml.Unmarshal([]byte(sample), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if c.Kind != "OnPremises" {
		t.Errorf("Kind did not decode, got %q", c.Kind)
	}

	if c.Spec.Kubernetes.Advanced == nil || c.Spec.Kubernetes.Advanced.Users == nil {
		t.Fatal("Spec.Kubernetes.Advanced.Users did not decode")
	}

	if got := c.Spec.Kubernetes.Advanced.Users.Names; len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Errorf("Spec.Kubernetes.Advanced.Users.Names did not decode, got %v", got)
	}
}
