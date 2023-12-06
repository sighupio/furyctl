// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package kfddistribution_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/support"
	"sigs.k8s.io/e2e-framework/support/kind"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
	osx "github.com/sighupio/furyctl/internal/x/os"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type conf struct {
	APIVersion string   `yaml:"apiVersion" validate:"required,api-version"`
	Kind       string   `yaml:"kind"       validate:"required,cluster-kind"`
	Metadata   confMeta `yaml:"metadata"   validate:"required"`
	Spec       confSpec `yaml:"spec"       validate:"required"`
}

type confSpec struct {
	DistributionVersion string `yaml:"distributionVersion" validate:"required"`
}

type confMeta struct {
	Name string `yaml:"name" validate:"required"`
}

type contextState struct {
	TestId      int
	TestName    string
	Cluster     support.E2EClusterProvider
	ClusterName string
	Kubeconfig  string
	HomeDir     string
	DataDir     string
	DistroDir   string
	TestDir     string
	TmpDir      string
}

func newContextState(testName string) *contextState {
	testId := rand.Intn(100000)
	clusterName := fmt.Sprintf("furytest-%d", testId)

	homeDir, dataDir, distroDir, tmpDir := PrepareDirs(testName)

	testDir := path.Join(homeDir, ".furyctl", "tests", testName)
	testState := path.Join(testDir, fmt.Sprintf("%s.teststate", clusterName))

	kubeconfig := path.Join(testDir, "kubeconfig")

	Must0(os.MkdirAll(testDir, 0o755))

	s := contextState{
		TestId:      testId,
		TestName:    testName,
		ClusterName: clusterName,
		Kubeconfig:  kubeconfig,
		HomeDir:     homeDir,
		DataDir:     dataDir,
		DistroDir:   distroDir,
		TestDir:     testDir,
		TmpDir:      tmpDir,
	}

	Must0(os.WriteFile(testState, Must1(json.Marshal(s)), 0o644))

	return &s
}

func TestExpensive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl Expensive Suite")
}

func Must0(err error) {
	if err != nil {
		panic(err)
	}
}

func Must1[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

var (
	furyctl string

	basePath = "../../data/expensive"

	binPath = filepath.Join(os.TempDir(), "bin")

	client = netx.NewGoGetterClient()

	distrodl = distribution.NewDownloader(client, true)

	toolFactory = tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{Bin: binPath})

	assertTimeout = 30 * time.Minute

	assertPollingInterval = 10 * time.Second

	_ = BeforeSuite(func() {
		tmpDir := Must1(os.MkdirTemp("", "furyctl-expensive-kfddistribution"))

		furyctl = filepath.Join(tmpDir, "furyctl")

		cmd := exec.Command("go", "build", "-o", furyctl, "../../../main.go")

		session := Must1(gexec.Start(cmd, GinkgoWriter, GinkgoWriter))

		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
	})

	DownloadFuryDistribution = func(furyctlConfPath string) distribution.DownloadResult {
		absBasePath := Must1(filepath.Abs(basePath))

		commonDir := path.Join(absBasePath, "common")

		dlRes := Must1(distrodl.Download(commonDir, furyctlConfPath))

		return dlRes
	}

	DownloadKubectl = func(version string) string {
		name := "kubectl"

		tfc := toolFactory.Create(tool.Name(name), version)
		if tfc == nil || !tfc.SupportsDownload() {
			panic(fmt.Errorf("tool '%s' does not support download", name))
		}

		dst := filepath.Join(binPath, name, version)

		if err := client.Download(tfc.SrcPath(), dst); err != nil {
			panic(fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, tfc.SrcPath(), err))
		}

		if err := tfc.Rename(dst); err != nil {
			panic(fmt.Errorf("%w '%s': %v", distribution.ErrRenamingFile, tfc.SrcPath(), err))
		}

		if err := os.Chmod(filepath.Join(dst, name), iox.FullPermAccess); err != nil {
			panic(fmt.Errorf("%w '%s': %v", distribution.ErrChangingFilePermissions, tfc.SrcPath(), err))
		}

		return path.Join(dst, name)
	}

	PrepareDirs = func(name string) (string, string, string, string) {
		absBasePath := Must1(filepath.Abs(basePath)) // TODO: get rid of this, ../../data/expensive

		homeDir := Must1(os.UserHomeDir())

		dataDir := Must1(filepath.Abs(path.Join(".", "testdata", name)))

		commonDir := path.Join(absBasePath, "common")

		tmpDir := Must1(os.MkdirTemp("", name))

		return homeDir, dataDir, commonDir, tmpDir
	}

	CreateFuryctlYaml = func(s *contextState, furyctlYamlTplName string) string {
		furyctlYamlTplPath := path.Join(s.DataDir, furyctlYamlTplName)

		tplData := Must1(os.ReadFile(furyctlYamlTplPath))

		data := bytes.ReplaceAll(tplData, []byte("__CLUSTER_NAME__"), []byte(s.ClusterName))

		furyctlYamlPath := path.Join(s.TestDir, fmt.Sprintf("%s.yaml", s.ClusterName))

		Must0(os.WriteFile(furyctlYamlPath, data, iox.FullPermAccess))

		return furyctlYamlPath
	}

	Copy = func(src, dst string) {
		input := Must1(os.ReadFile(src))

		Must0(os.WriteFile(dst, input, 0o644))
	}

	LoadFuryCtl = func(furyctlYamlPath string) conf {
		return Must1(yamlx.FromFileV3[conf](furyctlYamlPath))
	}

	FuryctlDeleteCluster = func(cfgPath, distroPath, phase string, dryRun bool, workDir string) *exec.Cmd {
		args := []string{
			"delete",
			"cluster",
			"--config",
			cfgPath,
			"--distro-location",
			distroPath,
			"--debug",
			"--force",
			"--workdir",
			workDir,
		}

		if phase != cluster.OperationPhaseAll {
			args = append(args, "--phase", phase)
		}

		if dryRun {
			args = append(args, "--dry-run")
		}

		return exec.Command(furyctl, args...)
	}

	FuryctlCreateCluster = func(configPath, distroPath, phase, skipPhase string, dryRun bool, workDir string) *exec.Cmd {
		args := []string{
			"create",
			"cluster",
			"--config",
			configPath,
			"--distro-location",
			distroPath,
			"--disable-analytics",
			"--debug",
			"--force",
			"--skip-vpn-confirmation",
			"--workdir",
			workDir,
		}

		if phase != cluster.OperationPhaseAll {
			args = append(args, "--phase", phase)
		}

		if phase == cluster.OperationPhaseInfrastructure {
			args = append(args, "--vpn-auto-connect")
		}

		if skipPhase != "" {
			args = append(args, "--skip-phase", skipPhase)
		}

		if dryRun {
			args = append(args, "--dry-run")
		}

		return exec.Command(furyctl, args...)
	}

	KillOpenVPN = func() (*gexec.Session, error) {
		var cmd *exec.Cmd

		isRoot, err := osx.IsRoot()
		if err != nil {
			return nil, err
		}

		if isRoot {
			cmd = exec.Command("pkill", "openvpn")
		} else {
			cmd = exec.Command("sudo", "pkill", "openvpn")
		}

		return gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	}

	_ = Describe("furyctl", Ordered, Serial, func() {
		_ = AfterEach(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Write([]byte("Test failed, cleaning up..."))
			}
		})

		Context("v1.25 create and delete", Ordered, Serial, Label("slow"), func() {
			var (
				furyctlYamlPath string
				state           *contextState
			)

			BeforeAll(func() {
				testName := "v1-25-create-and-delete"

				k := kind.NewProvider().SetDefaults().WithName(testName).WithOpts(kind.WithImage("kindest/node:v1.25.9"))

				kubecfg, err := k.CreateWithConfig(context.Background(), fmt.Sprintf("./testdata/%s/kind-config.yml", testName))
				Expect(err).To(Not(HaveOccurred()))

				Copy(kubecfg, fmt.Sprintf("./testdata/%s/kubeconfig", testName))

				state = newContextState(testName)

				state.Cluster = k

				GinkgoWriter.Write([]byte(fmt.Sprintf("Test id: %d", state.TestId)))

				Copy(fmt.Sprintf("./testdata/%s/kubeconfig", testName), state.Kubeconfig)

				os.Setenv("KUBECONFIG", state.Kubeconfig)

				client, err := klient.New(k.KubernetesRestConfig())
				Expect(err).To(Not(HaveOccurred()))

				err = k.WaitForControlPlane(context.Background(), client)
				Expect(err).To(Not(HaveOccurred()))

				furyctlYamlPath = CreateFuryctlYaml(state, "furyctl-minimal.yaml.tpl")
			})

			AfterAll(func() {
				state.Cluster.Destroy(context.Background())
			})

			It("should create a minimal 1.25 cluster", Serial, func() {
				dlRes := DownloadFuryDistribution(furyctlYamlPath)

				kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)

				GinkgoWriter.Write([]byte(fmt.Sprintf("Furyctl config path: %s", furyctlYamlPath)))

				createClusterCmd := FuryctlCreateCluster(
					furyctlYamlPath,
					state.DistroDir,
					cluster.OperationPhaseAll,
					"",
					false,
					state.TmpDir,
				)

				session := Must1(gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter))

				Consistently(session, 1*time.Minute).ShouldNot(gexec.Exit())

				Eventually(state.Kubeconfig, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
				Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubectlPath, "--kubeconfig", state.Kubeconfig, "get", "nodes")

				kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeSession, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
			})

			It("should delete a minimal 1.25 cluster", Serial, func() {
				deleteClusterCmd := FuryctlDeleteCluster(
					furyctlYamlPath,
					state.DistroDir,
					cluster.OperationPhaseAll,
					false,
					state.TmpDir,
				)

				session := Must1(gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter))
				Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
			})
		})
	})
)
