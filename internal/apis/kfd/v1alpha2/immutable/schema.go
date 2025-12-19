// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	netx "github.com/sighupio/furyctl/internal/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	// Basic validation errors.
	ErrClusterNameTooLong         = errors.New("cluster name must not exceed 40 characters")
	ErrNoNodesConfigured          = errors.New("at least one node must be configured in spec.infrastructure.nodes")
	ErrControlPlaneMemberNotFound = errors.New("control plane member references non-existent node")

	// Cross-reference validation errors.
	ErrEtcdMemberNotFound      = errors.New("etcd member references non-existent node")
	ErrNodeGroupMemberNotFound = errors.New("node group member references non-existent node")

	// HA validation errors.
	ErrControlPlaneMustBeOdd     = errors.New("control plane member count must be odd for HA quorum")
	ErrEtcdMustMatchControlPlane = errors.New("etcd member count must match control plane count")

	// Uniqueness validation errors.
	ErrDuplicateHostname   = errors.New("duplicate hostname found in infrastructure nodes")
	ErrDuplicateMacAddress = errors.New("duplicate MAC address found in infrastructure nodes")
	ErrDuplicateIPAddress  = errors.New("duplicate IP address found in node network configurations")

	// Exclusive membership validation error.
	ErrControlPlaneEtcdOverlap = errors.New("control plane and etcd members must be exclusive when using external etcd")

	// Network validation errors.
	ErrCIDROverlap        = errors.New("podCIDR and serviceCIDR must not overlap")
	ErrNodeNetworkOverlap = errors.New("kubernetes CIDRs must not overlap with node network IPs")
	ErrIPMismatch         = errors.New("IP in kubernetes section does not match infrastructure node configuration")
)

// ValidationSeverity represents the severity level of a validation issue.
type ValidationSeverity string

const (
	// ValidationSeverityError indicates a critical validation error that prevents deployment.
	ValidationSeverityError ValidationSeverity = "error"
	// ValidationSeverityWarning indicates a non-critical validation issue that allows deployment.
	ValidationSeverityWarning ValidationSeverity = "warning"
)

//nolint:errname // SchemaValidationIssue represents a validation issue with severity and context.
type SchemaValidationIssue struct {
	Severity ValidationSeverity
	Context  string // E.g., "Control Plane HA", "IP Consistency".
	Message  string // The actual error/warning message.
	Details  string // Optional: additional context.
}

// Error implements the error interface.
func (s SchemaValidationIssue) Error() string {
	if s.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", s.Context, s.Message, s.Details)
	}

	return fmt.Sprintf("%s: %s", s.Context, s.Message)
}

// validationCollector accumulates validation issues during schema validation.
type validationCollector struct {
	issues []SchemaValidationIssue
}

// newValidationCollector creates a new validation collector.
func newValidationCollector() *validationCollector {
	return &validationCollector{
		issues: make([]SchemaValidationIssue, 0),
	}
}

// addError adds a validation error to the collector.
func (c *validationCollector) addError(context, message, details string) {
	c.issues = append(c.issues, SchemaValidationIssue{
		Severity: ValidationSeverityError,
		Context:  context,
		Message:  message,
		Details:  details,
	})
}

// addWarning adds a validation warning to the collector.
func (c *validationCollector) addWarning(context, message, details string) {
	c.issues = append(c.issues, SchemaValidationIssue{
		Severity: ValidationSeverityWarning,
		Context:  context,
		Message:  message,
		Details:  details,
	})
}

// getErrors returns all error issues.
func (c *validationCollector) getErrors() []SchemaValidationIssue {
	errs := make([]SchemaValidationIssue, 0)

	for _, issue := range c.issues {
		if issue.Severity == ValidationSeverityError {
			errs = append(errs, issue)
		}
	}

	return errs
}

// getWarnings returns all warning issues.
func (c *validationCollector) getWarnings() []SchemaValidationIssue {
	warnings := make([]SchemaValidationIssue, 0)

	for _, issue := range c.issues {
		if issue.Severity == ValidationSeverityWarning {
			warnings = append(warnings, issue)
		}
	}

	return warnings
}

// logWarnings logs all warnings using logrus.
func (c *validationCollector) logWarnings() {
	warnings := c.getWarnings()
	if len(warnings) == 0 {
		return
	}

	logrus.Warnf("Found %d validation warning(s):", len(warnings))

	for _, warning := range warnings {
		logrus.Warnf("  - %v", warning)
	}
}

// firstError returns the first error or nil if no errors exist.
func (c *validationCollector) firstError() error {
	errs := c.getErrors()
	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%w", &errs[0])
}

type ExtraSchemaValidator struct{}

// Validate executes extra validation rules for Immutable kind.
func (v *ExtraSchemaValidator) Validate(confPath string) error {
	furyctlConf, err := yamlx.FromFileV3[public.ImmutableKfdV1Alpha2](confPath)
	if err != nil {
		return err
	}

	collector := newValidationCollector()

	// FASE 1: Fast-fail validations (critical basic checks).
	if err := v.validateClusterName(&furyctlConf); err != nil {
		return err
	}

	if err := v.validateNodes(&furyctlConf); err != nil {
		return err
	}

	// FASE 2: Uniqueness validations (fast-fail, critical).
	if err := v.validateUniqueHostnames(&furyctlConf); err != nil {
		return err
	}

	if err := v.validateUniqueMacAddresses(&furyctlConf); err != nil {
		return err
	}

	if err := v.validateUniqueIPAddresses(&furyctlConf); err != nil {
		return err
	}

	if err := v.validateExclusiveControlPlaneEtcd(&furyctlConf); err != nil {
		return err
	}

	// FASE 3: Network validations (fast-fail, critical).
	if err := v.validateCIDRNoOverlap(&furyctlConf); err != nil {
		return err
	}

	if err := v.validateNodeNetworkOverlap(&furyctlConf); err != nil {
		return err
	}

	// FASE 4: Cross-references (collect ALL errors, continue).
	v.validateControlPlaneMembersWithCollector(&furyctlConf, collector)
	v.validateEtcdMembersWithCollector(&furyctlConf, collector)
	v.validateNodeGroupMembersWithCollector(&furyctlConf, collector)

	// FASE 5: IP consistency (collect ALL errors, continue).
	v.validateIPConsistencyWithCollector(&furyctlConf, collector)

	// FASE 6: HA validations (WARNINGS only, never fail).
	v.validateControlPlaneHAWithCollector(&furyctlConf, collector)
	v.validateEtcdHAWithCollector(&furyctlConf, collector)

	// Log warnings (non-blocking).
	collector.logWarnings()

	// Return first error if any errors exist.
	return collector.firstError()
}

// validateClusterName checks cluster name length (max 40 chars for K8s label compatibility).
func (*ExtraSchemaValidator) validateClusterName(conf *public.ImmutableKfdV1Alpha2) error {
	const maxClusterNameLength = 40

	if len(conf.Metadata.Name) > maxClusterNameLength {
		return fmt.Errorf(
			"%w: '%s' has %d characters (max: %d)",
			ErrClusterNameTooLong,
			conf.Metadata.Name,
			len(conf.Metadata.Name),
			maxClusterNameLength,
		)
	}

	return nil
}

// validateNodes ensures at least one node is configured.
func (*ExtraSchemaValidator) validateNodes(conf *public.ImmutableKfdV1Alpha2) error {
	if len(conf.Spec.Infrastructure.Nodes) == 0 {
		return ErrNoNodesConfigured
	}

	return nil
}

// validateUniqueHostnames ensures all node hostnames are unique.
func (*ExtraSchemaValidator) validateUniqueHostnames(conf *public.ImmutableKfdV1Alpha2) error {
	seenHostnames := make(map[string]int, len(conf.Spec.Infrastructure.Nodes))

	for i, node := range conf.Spec.Infrastructure.Nodes {
		if firstIdx, exists := seenHostnames[node.Hostname]; exists {
			return fmt.Errorf(
				"%w: hostname '%s' appears at nodes[%d] and nodes[%d]",
				ErrDuplicateHostname,
				node.Hostname,
				firstIdx,
				i,
			)
		}

		seenHostnames[node.Hostname] = i
	}

	return nil
}

// validateUniqueMacAddresses ensures all PXE boot MAC addresses are unique across nodes.
func (*ExtraSchemaValidator) validateUniqueMacAddresses(conf *public.ImmutableKfdV1Alpha2) error {
	seenMacs := make(map[string]int, len(conf.Spec.Infrastructure.Nodes))

	for i, node := range conf.Spec.Infrastructure.Nodes {
		macLower := strings.ToLower(string(node.MacAddress))
		if firstIdx, exists := seenMacs[macLower]; exists {
			return fmt.Errorf(
				"%w: MAC address '%s' appears at nodes[%d] and nodes[%d]",
				ErrDuplicateMacAddress,
				node.MacAddress,
				firstIdx,
				i,
			)
		}

		seenMacs[macLower] = i
	}

	return nil
}

// validateUniqueIPAddresses ensures all IP addresses are unique across all node network configurations.
func (*ExtraSchemaValidator) validateUniqueIPAddresses(conf *public.ImmutableKfdV1Alpha2) error {
	seenIPs := make(map[string]string)
	nodeInterfaceIPs := collectNodeIPs(conf.Spec.Infrastructure.Nodes)

	for nodeIdx, interfaces := range nodeInterfaceIPs {
		for ifName, ips := range interfaces {
			for _, ip := range ips {
				if location, exists := seenIPs[ip]; exists {
					return fmt.Errorf(
						"%w: IP '%s' at nodes[%d].network.%s already used at %s",
						ErrDuplicateIPAddress,
						ip,
						nodeIdx,
						ifName,
						location,
					)
				}

				seenIPs[ip] = fmt.Sprintf("nodes[%d].network.%s", nodeIdx, ifName)
			}
		}
	}

	return nil
}

// validateExclusiveControlPlaneEtcd ensures control plane and etcd members are exclusive when using external etcd.
func (*ExtraSchemaValidator) validateExclusiveControlPlaneEtcd(conf *public.ImmutableKfdV1Alpha2) error {
	// If no etcd members configured, etcd runs with control plane (internal etcd, valid).
	if len(conf.Spec.Kubernetes.Etcd.Members) == 0 {
		return nil
	}

	// Build set of control plane hostnames.
	cpHostnames := make(map[string]int, len(conf.Spec.Kubernetes.ControlPlane.Members))
	for i, member := range conf.Spec.Kubernetes.ControlPlane.Members {
		cpHostnames[member.Hostname] = i
	}

	// Verify no etcd node is in control plane.
	for i, etcdMember := range conf.Spec.Kubernetes.Etcd.Members {
		if cpIdx, exists := cpHostnames[etcdMember.Hostname]; exists {
			return fmt.Errorf(
				"%w: hostname '%s' found in both controlPlane.members[%d] and etcd.members[%d] "+
					"(when using external etcd, nodes must be exclusive)",
				ErrControlPlaneEtcdOverlap,
				etcdMember.Hostname,
				cpIdx,
				i,
			)
		}
	}

	return nil
}

// validateCIDRNoOverlap ensures podCIDR and serviceCIDR do not overlap.
func (*ExtraSchemaValidator) validateCIDRNoOverlap(conf *public.ImmutableKfdV1Alpha2) error {
	podCIDR := string(conf.Spec.Kubernetes.Networking.PodCIDR)
	serviceCIDR := string(conf.Spec.Kubernetes.Networking.ServiceCIDR)

	overlaps, err := netx.CIDRsOverlap(podCIDR, serviceCIDR)
	if err != nil {
		return fmt.Errorf("failed to check CIDR overlap: %w", err)
	}

	if overlaps {
		return fmt.Errorf(
			"%w: podCIDR '%s' overlaps with serviceCIDR '%s'",
			ErrCIDROverlap,
			podCIDR,
			serviceCIDR,
		)
	}

	return nil
}

// validateNodeNetworkOverlap ensures node network IPs do not overlap with Kubernetes CIDRs.
func (*ExtraSchemaValidator) validateNodeNetworkOverlap(conf *public.ImmutableKfdV1Alpha2) error {
	podCIDR := string(conf.Spec.Kubernetes.Networking.PodCIDR)
	serviceCIDR := string(conf.Spec.Kubernetes.Networking.ServiceCIDR)

	nodeInterfaceIPs := collectNodeIPs(conf.Spec.Infrastructure.Nodes)

	for nodeIdx, interfaces := range nodeInterfaceIPs {
		for ifName, ips := range interfaces {
			for _, ip := range ips {
				inPodCIDR, err := netx.IPInCIDR(ip, podCIDR)
				if err != nil {
					continue
				}

				if inPodCIDR {
					return fmt.Errorf(
						"%w: IP '%s' in nodes[%d].network.%s falls within podCIDR '%s'",
						ErrNodeNetworkOverlap,
						ip,
						nodeIdx,
						ifName,
						podCIDR,
					)
				}

				inServiceCIDR, err := netx.IPInCIDR(ip, serviceCIDR)
				if err != nil {
					continue
				}

				if inServiceCIDR {
					return fmt.Errorf(
						"%w: IP '%s' in nodes[%d].network.%s falls within serviceCIDR '%s'",
						ErrNodeNetworkOverlap,
						ip,
						nodeIdx,
						ifName,
						serviceCIDR,
					)
				}
			}
		}
	}

	return nil
}

// validateControlPlaneMembersWithCollector verifies all control plane members reference existing nodes.
func (*ExtraSchemaValidator) validateControlPlaneMembersWithCollector(
	conf *public.ImmutableKfdV1Alpha2,
	collector *validationCollector,
) {
	validNodeHostnames := buildNodeHostnameSet(conf.Spec.Infrastructure.Nodes)

	for i, member := range conf.Spec.Kubernetes.ControlPlane.Members {
		if _, found := validNodeHostnames[member.Hostname]; !found {
			collector.addError(
				"Control Plane Member Reference",
				ErrControlPlaneMemberNotFound.Error(),
				fmt.Sprintf("member #%d hostname '%s' does not match any configured node", i, member.Hostname),
			)
		}
	}
}

// validateEtcdMembersWithCollector verifies all etcd members reference existing nodes.
func (*ExtraSchemaValidator) validateEtcdMembersWithCollector(
	conf *public.ImmutableKfdV1Alpha2,
	collector *validationCollector,
) {
	validNodeHostnames := buildNodeHostnameSet(conf.Spec.Infrastructure.Nodes)

	for i, member := range conf.Spec.Kubernetes.Etcd.Members {
		if _, found := validNodeHostnames[member.Hostname]; !found {
			collector.addError(
				"Etcd Member Reference",
				ErrEtcdMemberNotFound.Error(),
				fmt.Sprintf("member #%d hostname '%s' does not match any configured node", i, member.Hostname),
			)
		}
	}
}

// validateNodeGroupMembersWithCollector verifies all node group members reference existing nodes.
func (*ExtraSchemaValidator) validateNodeGroupMembersWithCollector(
	conf *public.ImmutableKfdV1Alpha2,
	collector *validationCollector,
) {
	validNodeHostnames := buildNodeHostnameSet(conf.Spec.Infrastructure.Nodes)

	for groupIdx, group := range conf.Spec.Kubernetes.NodeGroups {
		for nodeIdx, member := range group.Nodes {
			if _, found := validNodeHostnames[member.Hostname]; !found {
				collector.addError(
					"Node Group Member Reference",
					ErrNodeGroupMemberNotFound.Error(),
					fmt.Sprintf(
						"nodeGroups[%d] '%s' node #%d hostname '%s' does not match any configured node",
						groupIdx,
						group.Name,
						nodeIdx,
						member.Hostname,
					),
				)
			}
		}
	}
}

// validateIPConsistencyWithCollector ensures IPs in kubernetes sections match infrastructure node IPs.
func (*ExtraSchemaValidator) validateIPConsistencyWithCollector(
	conf *public.ImmutableKfdV1Alpha2,
	collector *validationCollector,
) {
	nodeIPs := buildNodeIPMap(conf.Spec.Infrastructure.Nodes)

	// Validate control plane member IPs.
	for i, member := range conf.Spec.Kubernetes.ControlPlane.Members {
		if member.Ip != nil {
			requestedIP := string(*member.Ip)
			if ips, found := nodeIPs[member.Hostname]; found {
				if !ips[requestedIP] {
					collector.addError(
						"IP Consistency - Control Plane",
						ErrIPMismatch.Error(),
						fmt.Sprintf(
							"controlPlane member #%d IP '%s' not found in node '%s' network configuration",
							i,
							requestedIP,
							member.Hostname,
						),
					)
				}
			}
		}
	}

	// Validate etcd member IPs.
	for i, member := range conf.Spec.Kubernetes.Etcd.Members {
		if member.Ip != nil {
			requestedIP := string(*member.Ip)
			if ips, found := nodeIPs[member.Hostname]; found {
				if !ips[requestedIP] {
					collector.addError(
						"IP Consistency - Etcd",
						ErrIPMismatch.Error(),
						fmt.Sprintf(
							"etcd member #%d IP '%s' not found in node '%s' network configuration",
							i,
							requestedIP,
							member.Hostname,
						),
					)
				}
			}
		}
	}

	// Validate node group member IPs.
	for groupIdx, group := range conf.Spec.Kubernetes.NodeGroups {
		for nodeIdx, member := range group.Nodes {
			if member.Ip != nil {
				requestedIP := string(*member.Ip)
				if ips, found := nodeIPs[member.Hostname]; found {
					if !ips[requestedIP] {
						collector.addError(
							"IP Consistency - Node Group",
							ErrIPMismatch.Error(),
							fmt.Sprintf(
								"nodeGroups[%d] '%s' node #%d IP '%s' not found in node '%s' network configuration",
								groupIdx,
								group.Name,
								nodeIdx,
								requestedIP,
								member.Hostname,
							),
						)
					}
				}
			}
		}
	}
}

// validateControlPlaneHAWithCollector validates control plane HA configuration.
func (*ExtraSchemaValidator) validateControlPlaneHAWithCollector(
	conf *public.ImmutableKfdV1Alpha2,
	collector *validationCollector,
) {
	count := len(conf.Spec.Kubernetes.ControlPlane.Members)

	if count%2 == 0 {
		collector.addWarning(
			"Control Plane High Availability",
			"Sub-optimal HA configuration detected",
			fmt.Sprintf(
				"found %d control plane members (even number). For proper quorum, use odd numbers (1, 3, 5, 7, ...)",
				count,
			),
		)
	}
}

// validateEtcdHAWithCollector validates etcd HA configuration.
func (*ExtraSchemaValidator) validateEtcdHAWithCollector(
	conf *public.ImmutableKfdV1Alpha2,
	collector *validationCollector,
) {
	controlPlaneCount := len(conf.Spec.Kubernetes.ControlPlane.Members)
	etcdCount := len(conf.Spec.Kubernetes.Etcd.Members)

	if etcdCount != controlPlaneCount {
		collector.addWarning(
			"Etcd High Availability",
			"Mismatched member counts detected",
			fmt.Sprintf(
				"found %d etcd members but %d control plane members. Best practice: these should match for consistent quorum",
				etcdCount,
				controlPlaneCount,
			),
		)
	}
}

// Helper functions for reducing code duplication.

// buildNodeHostnameSet creates a map of valid node hostnames.
func buildNodeHostnameSet(nodes []public.SpecInfrastructureNode) map[string]bool {
	hostnameSet := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		hostnameSet[node.Hostname] = true
	}

	return hostnameSet
}

// collectNodeIPs extracts all IPs from node network interfaces.
// Returns: map[nodeIdx]map[interfaceName][]ip for detailed tracking.
func collectNodeIPs(nodes []public.SpecInfrastructureNode) map[int]map[string][]string {
	nodeInterfaceIPs := make(map[int]map[string][]string, len(nodes))

	for nodeIdx, node := range nodes {
		nodeInterfaceIPs[nodeIdx] = make(map[string][]string)

		// Extract IPs from ethernet interfaces.
		for ethName, eth := range node.Network.Ethernets {
			for _, cidr := range eth.Addresses {
				ip, err := netx.ExtractIPFromCIDR(string(cidr))
				if err != nil {
					// Skip invalid CIDRs (will be caught by JSON schema validation).
					continue
				}

				nodeInterfaceIPs[nodeIdx][ethName] = append(nodeInterfaceIPs[nodeIdx][ethName], ip)
			}
		}

		// Extract IPs from bond interfaces.
		for bondName, bond := range node.Network.Bonds {
			for _, cidr := range bond.Addresses {
				ip, err := netx.ExtractIPFromCIDR(string(cidr))
				if err != nil {
					// Skip invalid CIDRs (will be caught by JSON schema validation).
					continue
				}

				nodeInterfaceIPs[nodeIdx][bondName] = append(nodeInterfaceIPs[nodeIdx][bondName], ip)
			}
		}
	}

	return nodeInterfaceIPs
}

// buildNodeIPMap creates hostname → IP set mapping for IP consistency validation.
func buildNodeIPMap(nodes []public.SpecInfrastructureNode) map[string]map[string]bool {
	nodeIPs := make(map[string]map[string]bool, len(nodes))

	for _, node := range nodes {
		nodeIPs[node.Hostname] = make(map[string]bool)

		// Collect IPs from ethernet interfaces.
		for _, eth := range node.Network.Ethernets {
			for _, cidr := range eth.Addresses {
				ip, err := netx.ExtractIPFromCIDR(string(cidr))
				if err != nil {
					continue
				}

				nodeIPs[node.Hostname][ip] = true
			}
		}

		// Collect IPs from bond interfaces.
		for _, bond := range node.Network.Bonds {
			for _, cidr := range bond.Addresses {
				ip, err := netx.ExtractIPFromCIDR(string(cidr))
				if err != nil {
					continue
				}

				nodeIPs[node.Hostname][ip] = true
			}
		}
	}

	return nodeIPs
}
