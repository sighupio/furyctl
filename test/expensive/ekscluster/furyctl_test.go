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

	"github.com/sighupio/furyctl/internal/cluster"
	. "github.com/sighupio/furyctl/test/utils"
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

	BeforeCreateDeleteTestFunc = func(state *ContextState, version, furyctlYamlTemplate string) func() {
		return func() {
			*state = NewContextState(fmt.Sprintf("ekscluster-v%s-create-and-delete", version))

			GinkgoWriter.Write([]byte(fmt.Sprintf("Test id: %d", state.TestID)))

			Copy("./testdata/id_ed25519", path.Join(state.TestDir, "id_ed25519"))
			Copy("./testdata/id_ed25519.pub", path.Join(state.TestDir, "id_ed25519.pub"))

			CreateFuryctlYaml(state, furyctlYamlTemplate)
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

			furyctlCreator := NewFuryctlCreator(
				furyctl,
				state.FuryctlYaml,
				state.TmpDir,
				false,
			)

			createClusterCmd := furyctlCreator.Create(
				cluster.OperationPhaseAll,
				"",
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

			furyctlDeleter := NewFuryctlDeleter(
				furyctl,
				state.FuryctlYaml,
				state.DistroDir,
				state.TmpDir,
				false,
			)

			deleteClusterCmd := furyctlDeleter.Delete(
				cluster.OperationPhaseAll,
			)

			session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)

			Expect(err).To(Not(HaveOccurred()))
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	CreateAndDeleteTestScenario = func(version string) func() {
		var state *ContextState = new(ContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal public cluster", version)

			Context(contextTitle, Ordered, Serial, Label("slow"), func() {
				BeforeAll(BeforeCreateDeleteTestFunc(state, version, "furyctl-public-public minimal.yaml.tpl"))

				It(fmt.Sprintf("should create a minimal public %s cluster", version), Serial, CreateClusterTestFunc(state))

				It(fmt.Sprintf("should delete a minimal public %s cluster", version), Serial, DeleteClusterTestFunc(state))
			})
		}
	}

	// _ = Describe("furyctl & distro v1.25.0 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.0"))
	// _ = Describe("furyctl & distro v1.25.1 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.1"))
	// _ = Describe("furyctl & distro v1.25.2 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.2"))
	// _ = Describe("furyctl & distro v1.25.3 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.3"))
	// _ = Describe("furyctl & distro v1.25.4 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.4"))
	// _ = Describe("furyctl & distro v1.25.5 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.5"))
	// _ = Describe("furyctl & distro v1.25.6 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.6"))
	// _ = Describe("furyctl & distro v1.25.7 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.7"))
	_ = Describe("furyctl & distro v1.25.8 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.8"))

	// _ = Describe("furyctl & distro v1.26.0 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.0"))
	// _ = Describe("furyctl & distro v1.26.1 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.1"))
	// _ = Describe("furyctl & distro v1.26.2 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.2"))
	_ = Describe("furyctl & distro v1.26.3 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.3"))

	// _ = Describe("furyctl & distro v1.27.0 - public minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.0"))
)
