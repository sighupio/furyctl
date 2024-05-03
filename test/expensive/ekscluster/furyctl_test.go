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

	PrepareCreateDeleteClusterTest = func(state *ContextState, version, furyctlYamlTemplate string) {
		*state = NewContextState(fmt.Sprintf("ekscluster-v%s-create-and-delete", version))

		GinkgoWriter.Write([]byte(fmt.Sprintf("Test id: %d", state.TestID)))

		Copy("./testdata/id_ed25519", path.Join(state.TestDir, "id_ed25519"))
		Copy("./testdata/id_ed25519.pub", path.Join(state.TestDir, "id_ed25519.pub"))

		CreateFuryctlYaml(state, furyctlYamlTemplate, nil)
	}

	CreateClusterTest = func(state *ContextState) {
		dlRes := DownloadFuryDistribution(state.TestDir, state.FuryctlYaml)

		tfPlanPath := path.Join(
			state.TestDir,
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
			state.TestDir,
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

	DeleteClusterTest = func(state *ContextState, ephemeral bool) {
		DeferCleanup(func() {
			_ = os.RemoveAll(state.TestDir)
		})

		if ephemeral {
			_ = os.RemoveAll(path.Join(state.TestDir, ".furyctl"))
		}

		furyctlDeleter := NewFuryctlDeleter(
			furyctl,
			state.FuryctlYaml,
			state.TestDir,
			false,
		)

		deleteClusterCmd := furyctlDeleter.Delete(
			cluster.OperationPhaseAll,
		)

		session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)

		Expect(err).To(Not(HaveOccurred()))
		Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
	}

	CreateAndDeleteTestScenario = func(version string, ephemeral bool) func() {
		var state *ContextState = new(ContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal public cluster", version)

			Context(contextTitle, Label("slow"), func() {
				It(fmt.Sprintf("should create and delete a minimal public %s cluster", version), func(s *ContextState) func() {
					return func() {
						PrepareCreateDeleteClusterTest(state, version, "furyctl-public-minimal.yaml.tpl")

						CreateClusterTest(s)
						DeleteClusterTest(s, ephemeral)
					}
				}(state))
			})
		}
	}

	_ = Describe("furyctl & distro v1.25.4 - public minimal", CreateAndDeleteTestScenario("1.25.4", false))
	_ = Describe("furyctl & distro v1.25.5 - public minimal", CreateAndDeleteTestScenario("1.25.5", false))
	_ = Describe("furyctl & distro v1.25.6 - public minimal", CreateAndDeleteTestScenario("1.25.6", false))
	_ = Describe("furyctl & distro v1.25.7 - public minimal", CreateAndDeleteTestScenario("1.25.7", false))
	_ = Describe("furyctl & distro v1.25.8 - public minimal", CreateAndDeleteTestScenario("1.25.8", false))
	_ = Describe("furyctl & distro v1.25.9 - public minimal", CreateAndDeleteTestScenario("1.25.9", false))
	_ = Describe("furyctl & distro v1.25.10 - public minimal", CreateAndDeleteTestScenario("1.25.10", false))

	_ = Describe("furyctl & distro v1.25.10 - public minimal - ephemeral", CreateAndDeleteTestScenario("1.25.10", true))

	_ = Describe("furyctl & distro v1.26.0 - public minimal", CreateAndDeleteTestScenario("1.26.0", false))
	_ = Describe("furyctl & distro v1.26.1 - public minimal", CreateAndDeleteTestScenario("1.26.1", false))
	_ = Describe("furyctl & distro v1.26.2 - public minimal", CreateAndDeleteTestScenario("1.26.2", false))
	_ = Describe("furyctl & distro v1.26.3 - public minimal", CreateAndDeleteTestScenario("1.26.3", false))
	_ = Describe("furyctl & distro v1.26.4 - public minimal", CreateAndDeleteTestScenario("1.26.4", false))
	_ = Describe("furyctl & distro v1.26.5 - public minimal", CreateAndDeleteTestScenario("1.26.5", false))
	_ = Describe("furyctl & distro v1.26.6 - public minimal", CreateAndDeleteTestScenario("1.26.6", false))

	_ = Describe("furyctl & distro v1.26.6 - public minimal - ephemeral", CreateAndDeleteTestScenario("1.26.6", true))

	_ = Describe("furyctl & distro v1.27.0 - public minimal", CreateAndDeleteTestScenario("1.27.0", false))
	_ = Describe("furyctl & distro v1.27.1 - public minimal", CreateAndDeleteTestScenario("1.27.1", false))
	_ = Describe("furyctl & distro v1.27.2 - public minimal", CreateAndDeleteTestScenario("1.27.2", false))
	_ = Describe("furyctl & distro v1.27.3 - public minimal", CreateAndDeleteTestScenario("1.27.3", false))
	_ = Describe("furyctl & distro v1.27.4 - public minimal", CreateAndDeleteTestScenario("1.27.4", false))
	_ = Describe("furyctl & distro v1.27.5 - public minimal", CreateAndDeleteTestScenario("1.27.5", false))

	_ = Describe("furyctl & distro v1.27.5 - public minimal - ephemeral", CreateAndDeleteTestScenario("1.27.5", true))

	_ = Describe("furyctl & distro v1.28.0 - public minimal", CreateAndDeleteTestScenario("1.28.0", false))

	_ = Describe("furyctl & distro v1.28.0 - public minimal - ephemeral", CreateAndDeleteTestScenario("1.28.0", true))

	_ = Describe("furyctl & distro v1.29.0 - public minimal", CreateAndDeleteTestScenario("1.29.0", false))

	_ = Describe("furyctl & distro v1.29.0 - public minimal - ephemeral", CreateAndDeleteTestScenario("1.29.0", true))
)
