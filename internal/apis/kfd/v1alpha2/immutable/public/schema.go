// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package public contains furyctl's curated, hand-maintained view of the
// Immutable furyctl.yaml (apiVersion kfd.sighup.io/v1alpha2).
//
// It models ONLY the fields furyctl actually reads. The full furyctl.yaml is
// validated at runtime against the JSON schema shipped by the distribution
// (see internal/config.Validate); fields not modeled here are still validated
// there and still reach the templates via the raw YAML, so omitting them is
// safe. Keep this struct readable: it should read like a furyctl.yaml skeleton.
package public

// Domain string types. They carry meaning (and keep the string(...) conversions
// at the call sites meaningful) while behaving as plain strings.
type (
	Kind       string // furyctl.yaml kind discriminator (Immutable)
	Arch       string // node CPU architecture (e.g. x86-64, arm64)
	MacAddress string // node MAC address used for PXE boot identification
	URI        string // URL-shaped string
)

// ImmutableKfdV1Alpha2 is furyctl's read-view of an Immutable config.
type ImmutableKfdV1Alpha2 struct {
	Kind Kind `yaml:"kind"`
	Spec Spec `yaml:"spec"`
}

type Spec struct {
	Infrastructure SpecInfrastructure `yaml:"infrastructure"`
	Kubernetes     SpecKubernetes     `yaml:"kubernetes"`
}

// --- infrastructure ---

type SpecInfrastructure struct {
	IpxeServer    *SpecInfrastructureIpxeServer    `yaml:"ipxeServer,omitempty"`
	LoadBalancers *SpecInfrastructureLoadBalancers `yaml:"loadBalancers,omitempty"`
	Nodes         []SpecInfrastructureNode         `yaml:"nodes"`
	Proxy         *SpecInfrastructureProxy         `yaml:"proxy,omitempty"`
	Ssh           SpecInfrastructureSSH            `yaml:"ssh"`
}

// SpecInfrastructureNode is referenced by name in furyctl (node ignition/boot
// generation), so this type name must stay stable.
//
// Only the fields furyctl reads in Go are modeled here. The node butane templates
// also consume several free-form sub-trees off the node (network.ethernets,
// storage.{files,links,directories,additionalDisks}, systemd.units, passwd,
// kernelParameters), but those are NOT modeled here on purpose: the butane phase
// hands the templates the raw node decoded straight from the furyctl.yaml (see
// create.rawNodesByHostname), so unmodeled fields reach the template intact.
// Adding typed fields for them would just risk dropping their own sub-fields the
// same way.
type SpecInfrastructureNode struct {
	Arch       Arch                          `yaml:"arch,omitempty"`
	Hostname   string                        `yaml:"hostname"`
	MacAddress MacAddress                    `yaml:"macAddress"`
	Storage    SpecInfrastructureNodeStorage `yaml:"storage"`
}

type SpecInfrastructureNodeStorage struct {
	InstallDisk string `yaml:"installDisk"`
}

type SpecInfrastructureIpxeServer struct {
	BindAddress         *string  `yaml:"bindAddress,omitempty"`
	BindPort            *int     `yaml:"bindPort,omitempty"`
	PostInstallCommands []string `yaml:"postInstallCommands,omitempty"`
	PreInstallCommands  []string `yaml:"preInstallCommands,omitempty"`
	Url                 URI      `yaml:"url"`
}

type SpecInfrastructureLoadBalancers struct {
	Members []Member `yaml:"members,omitempty"`
}

type SpecInfrastructureProxy struct {
	Http    *string `yaml:"http,omitempty"`
	Https   *string `yaml:"https,omitempty"`
	NoProxy *string `yaml:"noProxy,omitempty"`
}

type SpecInfrastructureSSH struct {
	PrivateKeyPath *string `yaml:"privateKeyPath,omitempty"`
	PublicKeyPath  *string `yaml:"publicKeyPath,omitempty"`
	Username       string  `yaml:"username"`
}

// --- kubernetes ---

type SpecKubernetes struct {
	Advanced     *SpecKubernetesAdvanced    `yaml:"advanced,omitempty"`
	ControlPlane SpecKubernetesControlPlane `yaml:"controlPlane"`
	Etcd         *SpecKubernetesEtcd        `yaml:"etcd,omitempty"`
}

type SpecKubernetesAdvanced struct {
	Users *SpecKubernetesAdvancedUsers `yaml:"users,omitempty"`
}

type SpecKubernetesAdvancedUsers struct {
	Names []string `yaml:"names,omitempty"`
}

type SpecKubernetesControlPlane struct {
	Members []Member `yaml:"members"`
}

type SpecKubernetesEtcd struct {
	Members []Member `yaml:"members"`
}

// Member is a node reference (control-plane, etcd or load-balancer). furyctl
// reads only the hostname.
type Member struct {
	Hostname string `yaml:"hostname"`
}
