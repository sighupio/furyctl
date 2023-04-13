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

func TestClient_GetIngresses(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	ingresses, err := client.GetIngresses()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	wantedIngresses := []kubernetes.Ingress{
		{"ingress-1", []string{"host-1"}},
		{"ingress-2", []string{"host-2"}},
		{"ingress-3", []string{"host-3"}},
	}
	if !cmp.Equal(ingresses, wantedIngresses) {
		t.Errorf("expected ingresses to be %v, got: %v", wantedIngresses, ingresses)
	}
}

func TestClient_GetPersistentVolumes(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	pvs, err := client.GetPersistentVolumes()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(pvs) == 0 {
		t.Errorf("expected pvs to be not empty")
	}

	wantPvs := []string{"pv-1", "pv-2", "pv-3"}
	if !cmp.Equal(pvs, wantPvs) {
		t.Errorf("expected pvs to be %v, got: %v", wantPvs, pvs)
	}
}

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

func TestClient_GetLoadBalancers(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	svcs, err := client.GetLoadBalancers()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	wantSvcs := []string{"svc-1", "svc-2", "svc-3"}
	if !cmp.Equal(svcs, wantSvcs) {
		t.Errorf("expected svcs to be %v, got: %v", wantSvcs, svcs)
	}
}

func TestClient_DeleteAllResources(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	out, err := client.DeleteAllResources("pod", "default")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	wantOut := `res "res-1" deleted`
	if out != wantOut {
		t.Errorf("expected output to be '%s', got: '%s'", wantOut, out)
	}
}

func TestClient_DeleteFromPath(t *testing.T) {
	t.Parallel()

	client := FakeClient(t)

	out, err := client.DeleteFromPath("test")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	wantOut := `res "res-1" deleted`
	if out != wantOut {
		t.Errorf("expected output to be '%s', got: '%s'", wantOut, out)
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
		"",
		true,
		true,
		true,
		execx.NewFakeExecutor(),
	)
}
