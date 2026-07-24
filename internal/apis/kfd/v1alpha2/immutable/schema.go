// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// roleListPaths names the config paths that assign roles, shared by the messages below.
const roleListPaths = ".spec.kubernetes.controlPlane.members, .spec.kubernetes.etcd.members, " +
	".spec.kubernetes.nodeGroups[].nodes or .spec.infrastructure.loadBalancers.members"

var (
	ErrNodeWithoutRole = errors.New(
		"every node defined in .spec.infrastructure.nodes must be assigned at least one role " +
			"(referenced in " + roleListPaths + ")",
	)
	ErrNodeNotDefined = errors.New(
		"every hostname referenced by a role (" + roleListPaths +
			") must have a matching entry in .spec.infrastructure.nodes",
	)
	ErrNodeMultipleReferences = errors.New(
		"a node must be referenced exactly once, but these hostnames appear more than once across " +
			roleListPaths +
			" (a node belongs to a single role, and to a single node group; for stacked etcd omit " +
			"the .spec.kubernetes.etcd block instead of repeating hostnames)",
	)
)

type ExtraSchemaValidator struct{}

func (*ExtraSchemaValidator) Validate(confPath string) error {
	conf, err := yamlx.FromFileV3[public.ImmutableKfdV1Alpha2](confPath)
	if err != nil {
		return err
	}

	// Cross-check node lists and role lists: every node has a role, every referenced
	// hostname is a defined node, and no hostname is referenced more than once. Report
	// them together to surface all issues at once.
	return errors.Join(
		validateNodeRoles(&conf),
		validateNodeReferences(&conf),
		validateSingleReference(&conf),
	)
}

// validateNodeRoles asserts every node in .spec.infrastructure.nodes is assigned
// a role; an unreferenced node has no role (public.NodeRole returns NodeRoleNone).
func validateNodeRoles(conf *public.ImmutableKfdV1Alpha2) error {
	var orphans []string

	for _, node := range conf.Spec.Infrastructure.Nodes {
		if conf.NodeRole(node.Hostname) == public.NodeRoleNone {
			orphans = append(orphans, node.Hostname)
		}
	}

	if len(orphans) > 0 {
		return fmt.Errorf("%w: %s", ErrNodeWithoutRole, strings.Join(orphans, ", "))
	}

	return nil
}

// validateNodeReferences asserts every hostname referenced by a role list has a
// matching entry in .spec.infrastructure.nodes.
func validateNodeReferences(conf *public.ImmutableKfdV1Alpha2) error {
	defined := make(map[string]struct{}, len(conf.Spec.Infrastructure.Nodes))
	for _, node := range conf.Spec.Infrastructure.Nodes {
		defined[node.Hostname] = struct{}{}
	}

	var (
		dangling []string
		seen     = make(map[string]struct{})
	)

	for _, ra := range conf.RoleAssignments() {
		if _, ok := defined[ra.Hostname]; ok {
			continue
		}

		// A hostname referenced more than once (see validateSingleReference) would
		// recur here; report it once.
		if _, dup := seen[ra.Hostname]; dup {
			continue
		}

		seen[ra.Hostname] = struct{}{}
		dangling = append(dangling, ra.Hostname)
	}

	if len(dangling) > 0 {
		return fmt.Errorf("%w: %s", ErrNodeNotDefined, strings.Join(dangling, ", "))
	}

	return nil
}

// validateSingleReference asserts no hostname is referenced more than once across
// all role lists (whether by two node groups, or by a control-plane host repeated
// under a dedicated etcd block).
func validateSingleReference(conf *public.ImmutableKfdV1Alpha2) error {
	count := make(map[string]int)

	var order []string

	for _, ra := range conf.RoleAssignments() {
		if count[ra.Hostname] == 0 {
			order = append(order, ra.Hostname)
		}

		count[ra.Hostname]++
	}

	var offenders []string

	for _, hostname := range order {
		if count[hostname] > 1 {
			offenders = append(offenders, hostname)
		}
	}

	if len(offenders) > 0 {
		return fmt.Errorf("%w: %s", ErrNodeMultipleReferences, strings.Join(offenders, ", "))
	}

	return nil
}
