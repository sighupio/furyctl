// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package state_test

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func TestStore_GetConfig(t *testing.T) {
	t.Parallel()

	store := state.Store{
		DistroPath:    "",
		ConfigPath:    "",
		WorkDir:       "",
		KubectlRunner: FakeClient(t),
	}

	cfg, err := store.GetConfig()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(cfg, []byte("test string")) {
		t.Errorf("expected config to be %v, got: %v", []byte("config: test string"), cfg)
	}
}

func TestStore_StoreConfig(t *testing.T) {
	t.Parallel()

	store := state.Store{
		DistroPath:    "",
		ConfigPath:    path.Join("test_data", "furyctl.yaml"),
		WorkDir:       "",
		KubectlRunner: FakeClient(t),
	}

	renderedConfig := map[string]any{}

	err := store.StoreConfig(renderedConfig)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestStore_StoreKFD(t *testing.T) {
	t.Parallel()

	store := state.Store{
		DistroPath:    path.Join("test_data"),
		ConfigPath:    "",
		WorkDir:       path.Join("test_data"),
		KubectlRunner: FakeClient(t),
	}

	err := store.StoreKFD()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
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
		case "apply":
			fmt.Fprintf(os.Stdout, "")

		case "get":
			resType := args[7]
			if args[5] == "-A" {
				resType = args[6]
			}

			if resType == "secret" {
				fmt.Fprintf(os.Stdout,
					"apiVersion: v1\ndata:\n config: dGVzdCBzdHJpbmc=\nkind: Secret\nmetadata:\n name: secret-1\n type: Opaque\n namespace: kube-system")
			}
		}

	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}

func FakeClient(t *testing.T) *kubectl.Runner {
	t.Helper()

	return kubectl.NewRunner(
		execx.NewFakeExecutor("TestHelperProcess"),
		kubectl.Paths{
			Kubectl: "kubectl",
		},
		true,
		true,
		true,
	)
}
