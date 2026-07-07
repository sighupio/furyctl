// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package public_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/public"
)

// TestAntiDrift decodes a furyctl.yaml that exercises every field furyctl reads
// from a KFDDistribution config and asserts they decode. If the distribution
// schema renames a key (and the curated struct's yaml tag drifts from it), the
// field stays zero and this test fails — the safety net for hand maintenance.
func TestAntiDrift(t *testing.T) {
	t.Parallel()

	const sample = `
kind: KFDDistribution
spec:
  distribution:
    kubeconfig: /tmp/kubeconfig
    modules:
      networking:
        type: none
`

	var c public.KfddistributionKfdV1Alpha2
	if err := yaml.Unmarshal([]byte(sample), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if c.Kind != "KFDDistribution" {
		t.Errorf("Kind did not decode, got %q", c.Kind)
	}

	if c.Spec.Distribution.Kubeconfig != "/tmp/kubeconfig" {
		t.Errorf("Spec.Distribution.Kubeconfig did not decode, got %q", c.Spec.Distribution.Kubeconfig)
	}

	if c.Spec.Distribution.Modules.Networking.Type != "none" {
		t.Errorf("Spec.Distribution.Modules.Networking.Type did not decode, got %q", c.Spec.Distribution.Modules.Networking.Type)
	}
}
