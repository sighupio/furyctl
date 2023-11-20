// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package ekscluster_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

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

func TestExpensive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl Expensive Suite")
}

func Must[T any](t T, err error) T {
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
		tmpDir := Must(os.MkdirTemp("", "furyctl-expensive"))

		furyctl = filepath.Join(tmpDir, "furyctl")

		cmd := exec.Command("go", "build", "-o", furyctl, "../../../main.go")

		session := Must(gexec.Start(cmd, GinkgoWriter, GinkgoWriter))

		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
	})

	DownloadFuryDistribution = func(furyctlConfPath string) distribution.DownloadResult {
		absBasePath := Must(filepath.Abs(basePath))

		commonDir := path.Join(absBasePath, "common")

		dlRes := Must(distrodl.Download(commonDir, furyctlConfPath))

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

		return dst
	}

	PrepareDirs = func(name string) (string, string, string, string) {
		absBasePath := Must(filepath.Abs(basePath))

		homeDir := Must(os.UserHomeDir())

		dataDir := path.Join(absBasePath, name)

		commonDir := path.Join(absBasePath, "common")

		tmpDir := Must(os.MkdirTemp("", name))

		return homeDir, dataDir, commonDir, tmpDir
	}

	CreateFuryctlYaml = func(furyctlYamlTplPath string) string {
		tplData := Must(os.ReadFile(furyctlYamlTplPath))

		id := strconv.Itoa(rand.Intn(100000))

		tmpDir := Must(os.MkdirTemp("", id))

		data := bytes.ReplaceAll(tplData, []byte("__ID__"), []byte(id))

		furyctlYamlPath := path.Join(tmpDir, "furyctl.yaml")

		if err := os.WriteFile(furyctlYamlPath, data, iox.FullPermAccess); err != nil {
			panic(err)
		}

		return furyctlYamlPath
	}

	LoadFuryCtl = func(furyctlYamlPath string) conf {
		return Must(yamlx.FromFileV3[conf](furyctlYamlPath))
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

		Context("v1.25 cluster creation and delete", Ordered, Serial, Label("slow"), func() {
			homeDir, dataDir, distroDir, tmpDir := PrepareDirs("v1-25-create-delete")
			furyctlYamlPath := CreateFuryctlYaml(path.Join(dataDir, "furyctl-public-minimal.yaml.tpl"))
			clusterName := LoadFuryCtl(furyctlYamlPath).Metadata.Name

			kubeconfigPath := path.Join(homeDir, ".furyctl", clusterName, cluster.OperationPhaseKubernetes, "terraform", "secrets", "kubeconfig")

			if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
				panic(err)
			}

			FIt("should create a minimal 1.25 cluster", Serial, func() {
				dlRes := DownloadFuryDistribution(furyctlYamlPath)

				kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)
				tfInfraPath := path.Join(homeDir, ".furyctl", clusterName, cluster.OperationPhaseInfrastructure, "terraform")
				tfPlanPath := path.Join(tfInfraPath, "plan", "terraform.plan")

				createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroDir, cluster.OperationPhaseAll, "", false, tmpDir)

				session := Must(gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter))

				Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())

				Eventually(tfPlanPath, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
				Eventually(kubeconfigPath, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
				Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubectlPath, "--kubeconfig", kubeconfigPath, "get", "nodes")

				kubeSession := Must(gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter))

				Eventually(kubeSession, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
			})

			It("should delete a minimal 1.25 cluster", Serial, func() {
				DeferCleanup(func() {
					_ = os.Unsetenv("KUBECONFIG")
					_ = os.RemoveAll(tmpDir)

					pkillSession := Must(KillOpenVPN())

					Eventually(pkillSession, assertPollingInterval, 1*time.Second).Should(gexec.Exit(0))
				})

				deleteClusterCmd := FuryctlDeleteCluster(furyctlYamlPath, distroDir, cluster.OperationPhaseAll, false, tmpDir)

				session := Must(gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter))

				Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
			})
		})

		// Context("cluster creation skipping infra phase, and cleanup", Ordered, Serial, Label("slow"), func() {
		// 	absWorkDirPath, absCommonPath, w := CreatePaths("create-skip-infra")

		// 	It("should create a cluster, skipping the infrastructure phase", Serial, func() {
		// 		furyctlYamlPath := path.Join(absWorkDirPath, "furyctl.yaml")
		// 		distroPath := absCommonPath

		// 		homeDir, err := os.UserHomeDir()
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		kubectlPath := path.Join(homeDir, ".furyctl", "bin", "kubectl", "1.24.7", "kubectl")
		// 		kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-si", cluster.OperationPhaseKubernetes, "terraform", "secrets", "kubeconfig")

		// 		createClusterInfraCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, cluster.OperationPhaseInfrastructure, "", false, w)

		// 		infraSession, err := gexec.Start(createClusterInfraCmd, GinkgoWriter, GinkgoWriter)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		Eventually(infraSession, 20*time.Minute).Should(gexec.Exit(0))

		// 		createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, cluster.OperationPhaseAll, cluster.OperationPhaseInfrastructure, false, w)

		// 		session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)
		// 		Expect(err).To(Not(HaveOccurred()))
		// 		Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())
		// 		Eventually(kcfgPath, 20*time.Minute).Should(BeAnExistingFile())
		// 		Eventually(session, 40*time.Minute).Should(gexec.Exit(0))

		// 		kubeCmd := exec.Command(kubectlPath, "--kubeconfig", kcfgPath, "get", "nodes")

		// 		kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

		// 		Expect(err).To(Not(HaveOccurred()))
		// 		Eventually(kubeSession, 2*time.Minute).Should(gexec.Exit(0))
		// 	})

		// 	It("should destroy a cluster", Serial, func() {
		// 		furyctlYamlPath := path.Join(absWorkDirPath, "furyctl.yaml")
		// 		distroPath := absCommonPath

		// 		homeDir, err := os.UserHomeDir()
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-si", cluster.OperationPhaseKubernetes, "terraform", "secrets", "kubeconfig")

		// 		err = os.Setenv("KUBECONFIG", kcfgPath)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		DeferCleanup(func() {
		// 			_ = os.Unsetenv("KUBECONFIG")
		// 			_ = os.RemoveAll(w)
		// 			pkillSession, err := KillOpenVPN()
		// 			Expect(err).To(Not(HaveOccurred()))
		// 			Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))
		// 		})

		// 		deleteClusterCmd := FuryctlDeleteCluster(furyctlYamlPath, distroPath, cluster.OperationPhaseAll, false, w)

		// 		session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		Eventually(session, 40*time.Minute).Should(gexec.Exit(0))
		// 	})
		// })

		// Context("cluster creation skipping kubernetes phase, and cleanup", Ordered, Serial, Label("slow"), func() {
		// 	absWorkDirPath, absCommonPath, w := CreatePaths("create-skip-kube")

		// 	It("should create a cluster, skipping the kubernetes phase", Serial, func() {
		// 		furyctlYamlPath := path.Join(absWorkDirPath, "furyctl.yaml")
		// 		distroPath := absCommonPath

		// 		homeDir, err := os.UserHomeDir()
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		kubectlPath := path.Join(homeDir, ".furyctl", "bin", "kubectl", "1.24.7", "kubectl")
		// 		kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-sk", cluster.OperationPhaseKubernetes, "terraform", "secrets", "kubeconfig")

		// 		createClusterKubeCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, cluster.OperationPhaseAll, cluster.OperationPhaseDistribution, false, w)

		// 		kubeSession, err := gexec.Start(createClusterKubeCmd, GinkgoWriter, GinkgoWriter)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		Eventually(kubeSession, 20*time.Minute).Should(gexec.Exit(0))

		// 		err = os.Setenv("KUBECONFIG", kcfgPath)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, cluster.OperationPhaseAll, cluster.OperationPhaseKubernetes, false, w)

		// 		session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)
		// 		Expect(err).To(Not(HaveOccurred()))
		// 		Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())
		// 		Eventually(session, 40*time.Minute).Should(gexec.Exit(0))

		// 		kubeCmd := exec.Command(kubectlPath, "--kubeconfig", kcfgPath, "get", "nodes")

		// 		kubectlSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

		// 		Expect(err).To(Not(HaveOccurred()))
		// 		Eventually(kubectlSession, 2*time.Minute).Should(gexec.Exit(0))
		// 	})

		// 	It("should destroy a cluster", Serial, func() {
		// 		furyctlYamlPath := path.Join(absWorkDirPath, "furyctl.yaml")
		// 		distroPath := absCommonPath

		// 		homeDir, err := os.UserHomeDir()
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-sk", cluster.OperationPhaseKubernetes, "terraform", "secrets", "kubeconfig")

		// 		err = os.Setenv("KUBECONFIG", kcfgPath)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		DeferCleanup(func() {
		// 			_ = os.Unsetenv("KUBECONFIG")
		// 			_ = os.RemoveAll(w)
		// 			pkillSession, err := KillOpenVPN()
		// 			Expect(err).To(Not(HaveOccurred()))
		// 			Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))
		// 		})

		// 		deleteClusterCmd := FuryctlDeleteCluster(furyctlYamlPath, distroPath, cluster.OperationPhaseAll, false, w)

		// 		session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)
		// 		Expect(err).To(Not(HaveOccurred()))

		// 		Eventually(session, 40*time.Minute).Should(gexec.Exit(0))
		// 	})
		// })
	})
)
