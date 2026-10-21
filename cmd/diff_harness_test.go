// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/git"
)

const (
	harnessKubectlVersion = "1.25.8"

	harnessKfd = `version: v1.25.1
modules:
  auth: v0.0.3
  aws: v2.2.0
  dr: v1.11.0
  ingress: v1.14.1
  logging: v3.1.3
  monitoring: v2.1.0
  opa: v1.8.0
  networking: v1.12.2
kubernetes:
  onpremises:
    version: 1.26.7
    installer: v1.28.7
furyctlSchemas:
  onpremises:
    - apiVersion: kfd.sighup.io/v1alpha2
      kind: OnPremises
tools:
  common:
    furyagent:
      version: 0.3.0
    kubectl:
      version: ` + harnessKubectlVersion + `
    kustomize:
      version: 3.5.3
    terraform:
      version: 0.15.4
    yq:
      version: 4.34.1
    helm:
      version: 3.12.3
    helmfile:
      version: 0.156.0
    kapp:
      version: 0.66.1
    ansible:
      version: 2.16.1
`

	harnessConfig = `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: lab
spec:
  distributionVersion: v1.25.1
  distribution:
    modules:
      ingress:
        nginx:
          type: dual
`
)

// diffRun is the result of one in-process run of `furyctl diff`.
type diffRun struct {
	// Stdout is what the command printed on the standard output, log lines excluded.
	Stdout string
	// Logs is the log lines, one per line, without time and colours.
	Logs string
	Err  error
}

// runDiff runs the diff command against a fake stored configuration: a fake kubectl prints it as the
// furyctl-config secret. extraConfig is appended to the configuration file and to the stored one. The command is the one the
// binary registers, under a parent that has the flags of the root command the diff command reads.
func runDiff(t *testing.T, extraConfig string, args ...string) diffRun {
	t.Helper()

	dir := t.TempDir()
	distro := filepath.Join(dir, "distro")
	bin := filepath.Join(dir, "bin")
	configPath := filepath.Join(dir, "furyctl.yaml")

	stored := strings.Replace(harnessConfig, "type: dual", "type: single", 1) + extraConfig
	secret := "apiVersion: v1\nkind: Secret\ndata:\n  config: " + base64.StdEncoding.EncodeToString([]byte(stored)) + "\n"

	writeFile(t, filepath.Join(distro, "kfd.yaml"), harnessKfd, 0o600)
	writeFile(t, configPath, harnessConfig+extraConfig, 0o600)
	writeFile(t, filepath.Join(bin, "kubectl", harnessKubectlVersion, "kubectl"),
		"#!/bin/sh\ncat <<'EOF'\n"+secret+"EOF\n", 0o755)

	// The work folder of the cluster exists once it was applied.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".furyctl", "lab"), os.FileMode(0o755)))

	viper.Reset()
	t.Cleanup(viper.Reset)

	root := &cobra.Command{Use: "furyctl", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolP("debug", "D", false, "")
	root.PersistentFlags().BoolP("no-tty", "T", true, "")
	root.PersistentFlags().StringP("workdir", "w", "", "")
	root.PersistentFlags().StringP("outdir", "o", dir, "")
	root.PersistentFlags().StringP("log", "l", "", "")
	root.PersistentFlags().VarP(&git.ProtocolFlag{Protocol: git.ProtocolHTTPS}, "git-protocol", "g", "")
	root.AddCommand(cmd.NewDiffCmd())

	require.NoError(t, viper.BindPFlags(root.PersistentFlags()))

	logs := &bytes.Buffer{}

	logrus.SetOutput(logs)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableColors: true})

	t.Cleanup(func() { logrus.SetOutput(os.Stderr) })

	root.SetArgs(append([]string{
		"diff", "--config", configPath, "--distro-location", distro, "--bin-path", bin,
	}, args...))

	var err error

	stdout := captureStdout(t, func() { _, err = root.ExecuteC() })

	return diffRun{Stdout: stdout, Logs: logs.String(), Err: err}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	old := os.Stdout
	os.Stdout = w

	done := make(chan string)

	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	os.Stdout = old
	require.NoError(t, w.Close())

	return <-done
}
