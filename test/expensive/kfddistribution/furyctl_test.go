// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package kfddistribution_test

import (
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

	. "github.com/sighupio/furyctl/test/utils"

	"github.com/sighupio/furyctl/internal/cluster"
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

	BeforeCreateDeleteTestFunc = func(state *distroContextState, version string) func() {
		return func() {
			testName := fmt.Sprintf("kfddistribution-v%s-create-and-delete", version)

			ctxState := NewContextState(testName)

			ctxState.Kubeconfig = path.Join(ctxState.TestDir, "kubeconfig")

			k := kind.NewProvider().SetDefaults().WithName(testName).WithOpts(kind.WithImage("kindest/node:v1.25.9"))

			kubecfg, err := k.CreateWithConfig(context.Background(), fmt.Sprintf("%s/kind-config.yml", ctxState.DataDir))
			Expect(err).To(Not(HaveOccurred()))

			*state = *newDistroContextState(k, &ctxState)

			Copy(kubecfg, fmt.Sprintf("%s/kubeconfig", ctxState.DataDir))

			GinkgoWriter.Write([]byte(fmt.Sprintf("Test id: %d", state.TestId)))

			Copy(fmt.Sprintf("%s/kubeconfig", ctxState.DataDir), state.Kubeconfig)

			os.Setenv("KUBECONFIG", state.Kubeconfig)

			client, err := klient.New(k.KubernetesRestConfig())
			Expect(err).To(Not(HaveOccurred()))

			err = k.WaitForControlPlane(context.Background(), client)
			Expect(err).To(Not(HaveOccurred()))

			CreateFuryctlYaml(state.ContextState, "furyctl-minimal.yaml.tpl")
		}
	}

	AfterCreateDeleteTestFunc = func(state *distroContextState) func() {
		return func() {
			state.Cluster.Destroy(context.Background())
		}
	}

	CreateClusterTestFunc = func(state *distroContextState) func() {
		return func() {
			dlRes := DownloadFuryDistribution(state.FuryctlYaml)

			kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)

			GinkgoWriter.Write([]byte(fmt.Sprintf("Furyctl config path: %s", state.FuryctlYaml)))

			createClusterCmd := FuryctlCreateCluster(
				furyctl,
				state.FuryctlYaml,
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
		}
	}

	DeleteClusterTestFunc = func(state *distroContextState) func() {
		return func() {
			deleteClusterCmd := FuryctlDeleteCluster(
				furyctl,
				state.FuryctlYaml,
				state.DistroDir,
				cluster.OperationPhaseAll,
				false,
				state.TmpDir,
			)

			session := Must1(gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter))
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	CreateAndDeleteTestScenario = func(version string) func() {
		var state *distroContextState = new(distroContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal public cluster", version)

			Context(contextTitle, Ordered, Serial, Label("slow"), func() {
				BeforeAll(BeforeCreateDeleteTestFunc(state, version))

				AfterAll(AfterCreateDeleteTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster", version), Serial, CreateClusterTestFunc(state))

				It(fmt.Sprintf("should delete a minimal %s cluster", version), Serial, DeleteClusterTestFunc(state))
			})
		}
	}

	_ = Describe("furyctl & distro v1.25.8 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.8"))
)
