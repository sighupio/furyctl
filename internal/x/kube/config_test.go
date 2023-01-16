// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubex_test

import (
	"os"
	"path"
	"testing"

	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	dirPath, err := os.MkdirTemp("", "test-kube-config-")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dirPath)

	data := []byte("test")

	p, err := kubex.CreateConfig(data, dirPath)
	if err != nil {
		t.Fatal(err)
	}

	wantPath := path.Join(dirPath, "kubeconfig")

	if p != wantPath {
		t.Fatalf("got %s, want %s", p, wantPath)
	}

	_, err = os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	if string(f) != string(data) {
		t.Fatalf("got %s, want %s", string(f), string(data))
	}
}

func TestSetConfigEnv(t *testing.T) {
	t.Parallel()

	dirPath, err := os.MkdirTemp("", "test-kube-config-")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dirPath)

	data := []byte("test")

	p, err := kubex.CreateConfig(data, dirPath)
	if err != nil {
		t.Fatal(err)
	}

	err = kubex.SetConfigEnv(p)
	if err != nil {
		t.Fatal(err)
	}

	wantPath := path.Join(dirPath, "kubeconfig")

	gotPath := os.Getenv("KUBECONFIG")

	if gotPath != wantPath {
		t.Fatalf("got %s, want %s", gotPath, wantPath)
	}
}

func TestCopyConfigToWorkDir(t *testing.T) {
	t.Parallel()

	dirPath, err := os.MkdirTemp("", "test-kube-config-")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dirPath)

	data := []byte("test")

	p, err := kubex.CreateConfig(data, dirPath)
	if err != nil {
		t.Fatal(err)
	}

	err = kubex.CopyConfigToWorkDir(p)
	if err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	wantPath := path.Join(wd, "kubeconfig")

	defer os.Remove(wantPath)

	_, err = os.Stat(wantPath)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(f) != string(data) {
		t.Fatalf("got %s, want %s", string(f), string(data))
	}
}
