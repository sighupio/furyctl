// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package ekscluster_test

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	. "github.com/sighupio/furyctl/test/utils"

	"github.com/sighupio/furyctl/internal/cluster"
)

func TestExpensive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl Expensive Suite")
}

var (
	furyctl = filepath.Join(Must1(os.MkdirTemp("", "furyctl-expensive-ekscluster")), "furyctl")

	assertTimeout = 30 * time.Minute

	assertPollingInterval = 10 * time.Second

	_ = BeforeSuite(CompileFuryctl(furyctl))

	BeforeCreateDeleteTestFunc = func(state *ContextState) func() {
		return func() {
			GinkgoWriter.Write([]byte(fmt.Sprintf("Test id: %d", state.TestId)))

			Copy("./testdata/id_ed25519", path.Join(state.TestDir, "id_ed25519"))
			Copy("./testdata/id_ed25519.pub", path.Join(state.TestDir, "id_ed25519.pub"))

			CreateFuryctlYaml(state, "furyctl-public-minimal.yaml.tpl")
		}
	}

	CreateClusterTestFunc = func(state *ContextState) func() {
		return func() {
			dlRes := DownloadFuryDistribution(state.FuryctlYaml)

			tfPlanPath := path.Join(
				state.HomeDir,
				".furyctl",
				state.ClusterName,
				cluster.OperationPhaseInfrastructure,
				"terraform",
				"plan",
				"terraform.plan",
			)

			createClusterCmd := FuryctlCreateCluster(
				furyctl,
				state.FuryctlYaml,
				cluster.OperationPhaseAll,
				"",
				false,
				state.TmpDir,
			)

			session := Must1(gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter))

			Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())

			Eventually(tfPlanPath, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
			Eventually(state.Kubeconfig, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))

			kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)
			kubeCmd := exec.Command(kubectlPath, "--kubeconfig", state.Kubeconfig, "get", "nodes")

			kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

			Expect(err).To(Not(HaveOccurred()))
			Eventually(kubeSession, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	DeleteClusterTestFunc = func(state *ContextState) func() {
		return func() {
			DeferCleanup(func() {
				_ = os.RemoveAll(state.TmpDir)

				pkillSession := Must1(KillOpenVPN())

				Eventually(pkillSession, 5*time.Minute, 1*time.Second).Should(gexec.Exit(0))
			})

			deleteClusterCmd := FuryctlDeleteCluster(
				furyctl,
				state.FuryctlYaml,
				state.DistroDir,
				cluster.OperationPhaseAll,
				false,
				state.TmpDir,
			)

			session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)

			Expect(err).To(Not(HaveOccurred()))
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	_ = Describe("furyctl", Ordered, func() {
		_ = AfterEach(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Write([]byte("Test failed, cleaning up..."))
			}
		})

		for _, version := range []string{
			"1.25.0",
			"1.25.1",
			"1.25.2",
			"1.25.3",
			"1.25.4",
			"1.25.5",
			"1.25.6",
			"1.25.7",
			"1.25.8",
		} {
			Context(fmt.Sprintf("v%s create and delete", version), Ordered, Serial, Label("slow"), func() {
				state := NewContextState(fmt.Sprintf("ekscluster-v%s-create-and-delete", version))

				BeforeAll(BeforeCreateDeleteTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster", version), Serial, CreateClusterTestFunc(state))

				It(fmt.Sprintf("should delete a minimal %s cluster", version), Serial, DeleteClusterTestFunc(state))
			})
		}

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
