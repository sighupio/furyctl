// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package kfddistribution_test

import (
	"bytes"
	"context"
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
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/support"
	"sigs.k8s.io/e2e-framework/support/kind"

	"github.com/sighupio/furyctl/internal/cluster"
	. "github.com/sighupio/furyctl/test/utils"
)

type distroContextState struct {
	*ContextState
	Cluster support.E2EClusterProvider
}

func newDistroContextState(cluster support.E2EClusterProvider, state *ContextState) *distroContextState {
	return &distroContextState{
		ContextState: state,
		Cluster:      cluster,
	}
}

func TestExpensive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl Expensive Suite")
}

var (
	furyctl = filepath.Join(Must1(os.MkdirTemp("", "furyctl-expensive-kfddistribution")), "furyctl")

	assertTimeout = 30 * time.Minute

	assertPollingInterval = 10 * time.Second

	_ = BeforeSuite(CompileFuryctl(furyctl))

	BeforeCreateDeleteTestFunc = func(state *distroContextState, version, kubernetesVersion string) func() {
		return func() {
			testName := fmt.Sprintf("kfddistribution-v%s-create-and-delete", version)

			ctxState := NewContextState(testName)

			k := kind.NewProvider().SetDefaults().WithName(testName).WithOpts(kind.WithImage(fmt.Sprintf("kindest/node:%s", kubernetesVersion)))

			kubecfg, err := k.CreateWithConfig(context.Background(), fmt.Sprintf("%s/kind-config.yml", ctxState.DataDir))
			Expect(err).To(Not(HaveOccurred()))

			*state = *newDistroContextState(k, &ctxState)

			Copy(kubecfg, fmt.Sprintf("%s/kubeconfig", ctxState.DataDir))

			GinkgoWriter.Write([]byte(fmt.Sprintf("Test id: %d", state.TestID)))

			Copy(fmt.Sprintf("%s/kubeconfig", ctxState.DataDir), state.Kubeconfig)

			os.Setenv("KUBECONFIG", state.Kubeconfig)

			client, err := klient.New(k.KubernetesRestConfig())
			Expect(err).To(Not(HaveOccurred()))

			err = k.WaitForControlPlane(context.Background(), client)
			Expect(err).To(Not(HaveOccurred()))

			CreateFuryctlYaml(state.ContextState, "furyctl-minimal.yaml.tpl", InjectKubeconfig(state.Kubeconfig))
		}
	}

	AfterCreateDeleteTestFunc = func(state *distroContextState) func() {
		return func() {
			state.Cluster.Destroy(context.Background())
		}
	}

	CreateClusterTestFunc = func(state *distroContextState, phase string) func() {
		return func() {
			dlRes := DownloadFuryDistribution(state.TestDir, state.FuryctlYaml)

			kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)

			GinkgoWriter.Write([]byte(fmt.Sprintf("Furyctl config path: %s", state.FuryctlYaml)))

			furyctlCreator := NewFuryctlCreator(
				furyctl,
				state.FuryctlYaml,
				state.TestDir,
				false,
			)

			createClusterCmd := furyctlCreator.Create(
				phase,
				"",
			)

			session := Must1(gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter))

			Consistently(session, 1*time.Minute).ShouldNot(gexec.Exit())

			Eventually(state.Kubeconfig, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))

			kubeCmd := exec.Command(kubectlPath, "--kubeconfig", state.Kubeconfig, "get", "nodes")

			kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

			Expect(err).To(Not(HaveOccurred()))
			Eventually(kubeSession, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	InjectKubeconfig = func(kubeconfigPath string) FuryctlYamlCreatorStrategy {
		return func(prevData []byte) []byte {
			data := bytes.ReplaceAll(prevData, []byte("__KUBECONFIG__"), []byte(fmt.Sprintf("%s", kubeconfigPath)))

			return data
		}
	}

	DeleteClusterTestFunc = func(state *distroContextState, phase string, ephemeral bool) func() {
		return func() {
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
				phase,
			)

			session := Must1(gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter))
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	CreateAndDeleteTestScenario = func(version, kuberneterVersion string, ephemeral bool) func() {
		var state *distroContextState = new(distroContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal public cluster", version)

			Context(contextTitle, Ordered, Serial, Label("slow"), func() {
				BeforeAll(BeforeCreateDeleteTestFunc(state, version, kuberneterVersion))

				AfterAll(AfterCreateDeleteTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster", version), Serial, CreateClusterTestFunc(state, cluster.OperationPhaseAll))

				It(fmt.Sprintf("should delete a minimal %s cluster", version), Serial, DeleteClusterTestFunc(state, cluster.OperationPhaseAll, ephemeral))
			})
		}
	}

	CreateAndDeleteByPhaseTestScenario = func(version, kuberneterVersion string, ephemeral bool) func() {
		var state *distroContextState = new(distroContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal public cluster by phase", version)

			Context(contextTitle, Ordered, Serial, Label("slow"), func() {
				BeforeAll(BeforeCreateDeleteTestFunc(state, version, kuberneterVersion))

				AfterAll(AfterCreateDeleteTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster by phase", version), Serial, CreateClusterTestFunc(state, cluster.OperationPhaseDistribution))

				It(fmt.Sprintf("should delete a minimal %s clusterby phase", version), Serial, DeleteClusterTestFunc(state, cluster.OperationPhaseDistribution, ephemeral))
			})
		}
	}

	_ = Describe("furyctl & distro v1.25.4 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.4", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.5 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.5", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.6 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.6", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.7 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.7", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.8 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.8", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.9 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.9", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.10 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.10", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.26.0 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.0", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.1 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.1", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.2 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.2", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.3 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.3", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.4 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.4", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.5 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.5", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.6 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.6", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.27.0 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.0", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.1 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.1", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.2 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.2", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.3 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.3", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.4 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.4", "v1.27.10", false))

	_ = Describe("furyctl & distro v1.27.5 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.5", "v1.27.10", false))

	_ = Describe("furyctl & distro v1.28.0 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.28.0", "v1.28.6", false))

	_ = Describe("furyctl & distro v1.27.4 - minimal - ephemeral", Ordered, Serial, CreateAndDeleteTestScenario("1.27.4", "v1.27.3", true))

	_ = Describe("furyctl & distro v1.25.4 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.4", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.5 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.5", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.6 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.6", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.7 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.7", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.9 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.9", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.25.10 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.10", "v1.25.11", false))

	_ = Describe("furyctl & distro v1.26.0 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.0", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.0 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.0", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.1 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.1", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.2 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.2", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.4 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.4", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.5 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.5", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.26.6 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.6", "v1.26.6", false))

	_ = Describe("furyctl & distro v1.27.0 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.0", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.1 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.1", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.2 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.2", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.3 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.3", "v1.27.3", false))

	_ = Describe("furyctl & distro v1.27.4 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.4", "v1.27.10", false))

	_ = Describe("furyctl & distro v1.27.5 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.5", "v1.27.10", false))

	_ = Describe("furyctl & distro v1.28.0 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.28.0", "v1.28.6", false))
)
