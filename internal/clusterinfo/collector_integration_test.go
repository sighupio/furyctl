// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterinfo_test

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

const (
	minimalFuryctlYAML = `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.34.0
`

	minimalKFDYAML = `version: v1.34.0
modules:
  auth: v0.6.1
  dr: v3.3.0
  ingress: v5.0.0
  logging: v5.3.0
  monitoring: v4.1.0
  opa: v1.16.0
  networking: v3.1.0
  tracing: v1.4.0
kubernetes:
  onpremises:
    version: 1.34.4
    installer: v1.34.4
`
)

func TestCollector_Collect(t *testing.T) {
	t.Parallel()

	collector := FakeCollector(t)

	info, err := collector.Collect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if info.ClusterName != "test-cluster" {
		t.Errorf("expected ClusterName %q, got %q", "test-cluster", info.ClusterName)
	}

	if info.SDVersion != "v1.34.0" {
		t.Errorf("expected SDVersion %q, got %q", "v1.34.0", info.SDVersion)
	}

	if info.SDKind != "OnPremises" {
		t.Errorf("expected SDKind %q, got %q", "OnPremises", info.SDKind)
	}

	if info.SDInstallerVersion != "v1.34.4" {
		t.Errorf("expected SDInstallerVersion %q, got %q", "v1.34.4", info.SDInstallerVersion)
	}

	if info.KubernetesVersion != "v1.34.4" {
		t.Errorf("expected KubernetesVersion %q, got %q", "v1.34.4", info.KubernetesVersion)
	}
}

//nolint:paralleltest // TestHelperProcess is a subprocess helper used by execx.NewFakeExecutor, not a real test.
func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "kubectl":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, `{"clientVersion":{"gitVersion":"v1.34.4"},"serverVersion":{"gitVersion":"v1.34.4"}}`)

		case "get":
			if args[5] == "-A" {
				// Nodes call: kubectl get -A nodes -o json.
				fmt.Fprintf(os.Stdout, `{"items":[{"metadata":{"name":"node-1","labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"capacity":{"cpu":"4","memory":"8Gi"}}}]}`)

				os.Exit(0)
			}

			// Namespaced call: kubectl get -n kube-system <type> <name> -o <format>.
			if len(args) < 9 {
				os.Exit(1)
			}

			resourceType := args[7]
			resourceName := args[8]

			switch resourceType {
			case "secret":
				switch resourceName {
				case "furyctl-config":
					encoded := base64.StdEncoding.EncodeToString([]byte(minimalFuryctlYAML))
					fmt.Fprintf(os.Stdout, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: furyctl-config\n  namespace: kube-system\ndata:\n  config: %s\n", encoded)

				case "furyctl-kfd":
					encoded := base64.StdEncoding.EncodeToString([]byte(minimalKFDYAML))
					fmt.Fprintf(os.Stdout, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: furyctl-kfd\n  namespace: kube-system\ndata:\n  kfd: %s\n", encoded)

				default:
					os.Exit(1)
				}

			default:
				os.Exit(1)
			}

		default:
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}

func FakeCollector(t *testing.T) *clusterinfo.Collector {
	t.Helper()

	return &clusterinfo.Collector{
		KubectlRunner: kubectl.NewRunner(
			execx.NewFakeExecutor("TestHelperProcess"),
			kubectl.Paths{
				Kubectl: "kubectl",
			},
			false,
			true,
			false,
		),
	}
}
