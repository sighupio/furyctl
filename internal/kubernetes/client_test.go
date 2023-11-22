// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubernetes_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sighupio/furyctl/internal/kubernetes"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func TestClient_ListNamespaceResources(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	resources, err := client.ListNamespaceResources("pod", "default")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	wantedResources := []kubernetes.Resource{
		{Kind: "Pod", Name: "pod-1"},
		{Kind: "Pod", Name: "pod-2"},
		{Kind: "Pod", Name: "pod-3"},
	}
	if !cmp.Equal(resources, wantedResources) {
		t.Errorf("expected resources to be %v, got: %v", wantedResources, resources)
	}
}

func TestClient_ToolVersion(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	version, err := client.ToolVersion()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(version) == 0 {
		t.Errorf("expected version to be not empty")
	}
}

func TestHelperProcess(t *testing.T) {
	t.Parallel()

	t.Helper()

	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "kubectl":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "{\n"+
				"\"clientVersion\": {\n"+
				"\"major\": \"1\",\n"+
				"\"minor\": \"21\",\n"+
				"\"gitVersion\": \"v1.21.1\",\n"+
				"\"gitCommit\": \"xxxxx\",\n"+
				"\"gitTreeState\": \"clean\",\n"+
				"\"buildDate\": \"2021-05-12T14:00:00Z\",\n"+
				"\"goVersion\": \"go1.16.4\",\n"+
				"\"compiler\": \"gc\",\n"+
				"\"platform\": \"darwin/amd64\"\n"+
				"}\n"+
				"}\n")

		case "get":
			resType := args[7]
			if args[5] == "-A" {
				resType = args[6]
			}

			switch resType {
			case "pv":
				fmt.Fprintf(os.Stdout, "'pv-1 pv-2 pv-3'")

			case "svc":
				fmt.Fprintf(os.Stdout, "'svc-1 svc-2 svc-3'")

			case "ingress":
				fmt.Fprintf(os.Stdout,
					"'[{\"Name\": \"ingress-1\", \"Host\": [\"host-1\"]},"+
						"{\"Name\": \"ingress-2\", \"Host\": [\"host-2\"]},"+
						"{\"Name\": \"ingress-3\", \"Host\": [\"host-3\"]}]'")

			case "pod":
				fmt.Fprintf(os.Stdout,
					"'[{\"Name\": \"pod-1\", \"Kind\": \"Pod\"},"+
						"{\"Name\": \"pod-2\", \"Kind\": \"Pod\"},"+
						"{\"Name\": \"pod-3\", \"Kind\": \"Pod\"}]'")
			}

		case "delete":
			fmt.Fprintf(os.Stdout, "res \"res-1\" deleted")
		}

	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}

func FakeClient(t *testing.T) *kubernetes.Client {
	t.Helper()

	return kubernetes.NewClient(
		"kubectl",
		"",
		true,
		true,
		true,
		execx.NewFakeExecutor("TestHelperProcess"),
	)
}
