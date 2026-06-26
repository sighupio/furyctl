// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package public_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
)

// TestAntiDrift decodes a furyctl.yaml exercising the fields furyctl reads from
// an Immutable config and asserts they decode (guards yaml-tag drift from the
// distribution schema). The free-form node sub-trees the butane templates consume
// (network, storage extras, systemd and passwd) are intentionally not modeled on
// this struct: they reach the templates via the raw furyctl.yaml handled by
// create.rawNodesByHostname, so this test does not assert them.
func TestAntiDrift(t *testing.T) {
	t.Parallel()

	const sample = `
kind: Immutable
spec:
  infrastructure:
    ipxeServer:
      url: https://ipxe.example.com:8080
      preInstallCommands: ["echo pre"]
    loadBalancers:
      members:
        - hostname: lb01.example.com
    nodes:
      - hostname: node01.example.com
        macAddress: 52:54:00:10:00:01
        arch: x86-64
        storage:
          installDisk: /dev/sda
    proxy:
      http: http://proxy:3128
    ssh:
      username: core
      privateKeyPath: /key
  kubernetes:
    advanced:
      users:
        names: [admin]
    controlPlane:
      members:
        - hostname: ctrl01.example.com
    etcd:
      members:
        - hostname: etcd01.example.com
`

	var c public.ImmutableKfdV1Alpha2
	if err := yaml.Unmarshal([]byte(sample), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if c.Kind != "Immutable" {
		t.Errorf("Kind did not decode, got %q", c.Kind)
	}

	if len(c.Spec.Infrastructure.Nodes) != 1 {
		t.Fatalf("Spec.Infrastructure.Nodes did not decode, got %v", c.Spec.Infrastructure.Nodes)
	}

	node := c.Spec.Infrastructure.Nodes[0]
	if node.Hostname != "node01.example.com" || node.MacAddress != "52:54:00:10:00:01" ||
		node.Arch != "x86-64" || node.Storage.InstallDisk != "/dev/sda" {
		t.Errorf("node fields did not decode, got %+v", node)
	}

	if c.Spec.Infrastructure.IpxeServer == nil || c.Spec.Infrastructure.IpxeServer.Url != "https://ipxe.example.com:8080" {
		t.Errorf("IpxeServer.Url did not decode")
	}

	if c.Spec.Infrastructure.Proxy == nil || c.Spec.Infrastructure.Proxy.Http == nil {
		t.Errorf("Proxy.Http did not decode")
	}

	if c.Spec.Infrastructure.Ssh.Username != "core" {
		t.Errorf("Ssh.Username did not decode, got %q", c.Spec.Infrastructure.Ssh.Username)
	}

	if len(c.Spec.Kubernetes.ControlPlane.Members) != 1 || c.Spec.Kubernetes.ControlPlane.Members[0].Hostname != "ctrl01.example.com" {
		t.Errorf("ControlPlane.Members did not decode")
	}

	if c.Spec.Kubernetes.Etcd == nil || len(c.Spec.Kubernetes.Etcd.Members) != 1 {
		t.Errorf("Etcd.Members did not decode")
	}

	if c.Spec.Kubernetes.Advanced == nil || c.Spec.Kubernetes.Advanced.Users == nil ||
		len(c.Spec.Kubernetes.Advanced.Users.Names) != 1 {
		t.Errorf("Advanced.Users.Names did not decode")
	}

	if len(c.Spec.Infrastructure.LoadBalancers.Members) != 1 {
		t.Errorf("LoadBalancers.Members did not decode")
	}
}
