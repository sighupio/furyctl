// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package immutable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable"
)

func Test_ExtraSchemaValidator_Validate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		confPath   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			desc:     "every node is assigned at least one role",
			confPath: "test/schema/all_nodes_have_role.yaml",
		},
		{
			desc:     "a node is not assigned any role",
			confPath: "test/schema/node_without_role.yaml",
			wantErr:  true,
			wantErrMsg: "every node defined in .spec.infrastructure.nodes must be assigned at least one role " +
				"(referenced in .spec.kubernetes.controlPlane.members, .spec.kubernetes.etcd.members, " +
				".spec.kubernetes.nodeGroups[].nodes or .spec.infrastructure.loadBalancers.members): orphan01",
		},
		{
			desc:     "multiple nodes are not assigned any role",
			confPath: "test/schema/multiple_nodes_without_role.yaml",
			wantErr:  true,
			wantErrMsg: "every node defined in .spec.infrastructure.nodes must be assigned at least one role " +
				"(referenced in .spec.kubernetes.controlPlane.members, .spec.kubernetes.etcd.members, " +
				".spec.kubernetes.nodeGroups[].nodes or .spec.infrastructure.loadBalancers.members): orphan01, orphan02",
		},
		{
			desc:     "a referenced hostname has no matching node",
			confPath: "test/schema/dangling_node_reference.yaml",
			wantErr:  true,
			wantErrMsg: "every hostname referenced by a role (.spec.kubernetes.controlPlane.members, " +
				".spec.kubernetes.etcd.members, .spec.kubernetes.nodeGroups[].nodes or " +
				".spec.infrastructure.loadBalancers.members) must have a matching entry in " +
				".spec.infrastructure.nodes: ghost01",
		},
		{
			desc:     "multiple referenced hostnames have no matching node",
			confPath: "test/schema/multiple_dangling_node_references.yaml",
			wantErr:  true,
			wantErrMsg: "every hostname referenced by a role (.spec.kubernetes.controlPlane.members, " +
				".spec.kubernetes.etcd.members, .spec.kubernetes.nodeGroups[].nodes or " +
				".spec.infrastructure.loadBalancers.members) must have a matching entry in " +
				".spec.infrastructure.nodes: ghost01, ghost02",
		},
		{
			desc:     "both an unassigned node and a dangling reference are reported",
			confPath: "test/schema/orphan_and_dangling.yaml",
			wantErr:  true,
			wantErrMsg: "every node defined in .spec.infrastructure.nodes must be assigned at least one role " +
				"(referenced in .spec.kubernetes.controlPlane.members, .spec.kubernetes.etcd.members, " +
				".spec.kubernetes.nodeGroups[].nodes or .spec.infrastructure.loadBalancers.members): orphan01\n" +
				"every hostname referenced by a role (.spec.kubernetes.controlPlane.members, " +
				".spec.kubernetes.etcd.members, .spec.kubernetes.nodeGroups[].nodes or " +
				".spec.infrastructure.loadBalancers.members) must have a matching entry in " +
				".spec.infrastructure.nodes: ghost01",
		},
		{
			desc:     "a hostname is referenced by more than one role",
			confPath: "test/schema/node_with_multiple_roles.yaml",
			wantErr:  true,
			wantErrMsg: "a node must be assigned a single role, but these hostnames are referenced by more than " +
				"one of .spec.kubernetes.controlPlane.members, .spec.kubernetes.etcd.members, " +
				".spec.kubernetes.nodeGroups[].nodes or .spec.infrastructure.loadBalancers.members " +
				"(for stacked etcd omit the .spec.kubernetes.etcd block instead of repeating hostnames): " +
				"ctrl01 (controlplane, etcd)",
		},
		{
			desc:     "furyctl config is invalid",
			confPath: "test/schema/invalid.yaml",
			wantErr:  true,
		},
	}

	esv := &immutable.ExtraSchemaValidator{}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			err := esv.Validate(tC.confPath)

			if tC.wantErr {
				require.Error(t, err)

				if tC.wantErrMsg != "" {
					assert.Equal(t, tC.wantErrMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
