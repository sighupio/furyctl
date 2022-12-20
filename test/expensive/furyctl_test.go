// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package expensive_test

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestExpensive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl Expensive Suite")
}

var (
	furyctl string

	basePath = "../data/expensive"

	_ = BeforeSuite(func() {
		tmpdir, err := os.MkdirTemp("", "furyctl-expensive")
		Expect(err).To(Not(HaveOccurred()))

		furyctl = filepath.Join(tmpdir, "furyctl")

		cmd := exec.Command("go", "build", "-o", furyctl, "../../main.go")

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).To(Not(HaveOccurred()))
		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
	})

	CreatePaths = func(dir string) (string, string, string) {
		absBasePath, err := filepath.Abs(basePath)
		Expect(err).To(Not(HaveOccurred()))

		common := path.Join(absBasePath, "common")

		workDir := path.Join(absBasePath, dir)

		w, err := os.MkdirTemp("", "create-cluster-test-")
		Expect(err).To(Not(HaveOccurred()))

		Expect(w).To(BeADirectory())

		return workDir, common, w
	}

	FuryctlDeleteCluster = func(cfgPath, distroPath, phase string, dryRun bool, w string) *exec.Cmd {
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
			w,
		}

		if phase != "" {
			args = append(args, "--phase", phase)
		}

		if dryRun {
			args = append(args, "--dry-run")
		}

		return exec.Command(furyctl, args...)
	}

	FuryctlCreateCluster = func(cfgPath, distroPath, phase, skipPhase string, dryRun bool, w string) *exec.Cmd {
		args := []string{
			"create",
			"cluster",
			"--config",
			cfgPath,
			"--distro-location",
			distroPath,
			"--debug",
			"--workdir",
			w,
		}

		if phase != "" {
			args = append(args, "--phase", phase)
		}

		if phase == "infrastructure" {
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
		cmd := exec.Command("sudo", "pkill", "openvpn")

		return gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	}

	_ = Describe("furyctl", Ordered, Serial, func() {
		_ = AfterEach(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Write([]byte("Test failed, cleaning up..."))
			}
		})

		Context("cluster creation and cleanup", Ordered, Serial, Label("slow"), func() {
			absWorkDirPath, absCommonPath, w := CreatePaths("create-complete")

			It("should create a complete cluster", Serial, func() {
				furyctlYamlPath := path.Join(absWorkDirPath, "data/furyctl.yaml")
				distroPath := path.Join(absCommonPath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				kubeBinPath := path.Join(homeDir, ".furyctl", "bin", "kubectl", "1.23.7", "kubectl")
				tfInfraPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws", "infrastructure", "terraform")
				kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws", "kubernetes", "terraform", "secrets", "kubeconfig")

				createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "", "", false, w)

				session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))
				Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())
				Expect(path.Join(tfInfraPath, "plan", "terraform.plan")).To(BeAnExistingFile())
				Eventually(kcfgPath, 20*time.Minute).Should(BeAnExistingFile())
				Eventually(session, 40*time.Minute).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubeBinPath, "--kubeconfig", kcfgPath, "get", "nodes")

				kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeSession, 2*time.Minute).Should(gexec.Exit(0))
			})

			It("should destroy a cluster", Serial, func() {
				furyctlYamlPath := path.Join(absWorkDirPath, "data/furyctl.yaml")
				distroPath := path.Join(absCommonPath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws", "kubernetes", "terraform", "secrets", "kubeconfig")

				err = os.Setenv("KUBECONFIG", kcfgPath)
				Expect(err).To(Not(HaveOccurred()))

				DeferCleanup(func() {
					_ = os.Unsetenv("KUBECONFIG")
					_ = os.RemoveAll(w)
					pkillSession, err := KillOpenVPN()
					Expect(err).To(Not(HaveOccurred()))
					Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))
				})

				deleteClusterCmd := FuryctlDeleteCluster(furyctlYamlPath, distroPath, "", false, w)

				session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(session, 40*time.Minute).Should(gexec.Exit(0))
			})
		})

		Context("cluster creation skipping infra phase, and cleanup", Ordered, Serial, Label("slow"), func() {
			absWorkDirPath, absCommonPath, w := CreatePaths("create-skip-infra")

			It("should create a cluster, skipping the infrastructure phase", Serial, func() {
				furyctlYamlPath := path.Join(absWorkDirPath, "data/furyctl.yaml")
				distroPath := path.Join(absCommonPath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				kubeBinPath := path.Join(homeDir, ".furyctl", "bin", "kubectl", "1.23.7", "kubectl")
				kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-si", "kubernetes", "terraform", "secrets", "kubeconfig")

				createClusterInfraCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "infrastructure", "", false, w)

				infraSession, err := gexec.Start(createClusterInfraCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(infraSession, 20*time.Minute).Should(gexec.Exit(0))

				createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "", "infrastructure", false, w)

				session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))
				Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())
				Eventually(kcfgPath, 20*time.Minute).Should(BeAnExistingFile())
				Eventually(session, 40*time.Minute).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubeBinPath, "--kubeconfig", kcfgPath, "get", "nodes")

				kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeSession, 2*time.Minute).Should(gexec.Exit(0))
			})

			It("should destroy a cluster", Serial, func() {
				furyctlYamlPath := path.Join(absWorkDirPath, "data/furyctl.yaml")
				distroPath := path.Join(absCommonPath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-si", "kubernetes", "terraform", "secrets", "kubeconfig")

				err = os.Setenv("KUBECONFIG", kcfgPath)
				Expect(err).To(Not(HaveOccurred()))

				DeferCleanup(func() {
					_ = os.Unsetenv("KUBECONFIG")
					_ = os.RemoveAll(w)
					pkillSession, err := KillOpenVPN()
					Expect(err).To(Not(HaveOccurred()))
					Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))
				})

				deleteClusterCmd := FuryctlDeleteCluster(furyctlYamlPath, distroPath, "", false, w)

				session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(session, 40*time.Minute).Should(gexec.Exit(0))
			})
		})

		Context("cluster creation skipping kubernetes phase, and cleanup", Ordered, Serial, Label("slow"), func() {
			absWorkDirPath, absCommonPath, w := CreatePaths("create-skip-kube")

			It("should create a cluster, skipping the kubernetes phase", Serial, func() {
				furyctlYamlPath := path.Join(absWorkDirPath, "data/furyctl.yaml")
				distroPath := path.Join(absCommonPath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				kubeBinPath := path.Join(homeDir, ".furyctl", "bin", "kubectl", "1.23.7", "kubectl")
				kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-sk", "kubernetes", "terraform", "secrets", "kubeconfig")

				createClusterKubeCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "", "distribution", false, w)

				kubeSession, err := gexec.Start(createClusterKubeCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(kubeSession, 20*time.Minute).Should(gexec.Exit(0))

				err = os.Setenv("KUBECONFIG", kcfgPath)
				Expect(err).To(Not(HaveOccurred()))

				createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "", "kubernetes", false, w)

				session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))
				Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())
				Eventually(session, 40*time.Minute).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubeBinPath, "--kubeconfig", kcfgPath, "get", "nodes")

				kubectlSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubectlSession, 2*time.Minute).Should(gexec.Exit(0))
			})

			It("should destroy a cluster", Serial, func() {
				furyctlYamlPath := path.Join(absWorkDirPath, "data/furyctl.yaml")
				distroPath := path.Join(absCommonPath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				kcfgPath := path.Join(homeDir, ".furyctl", "furyctl-test-aws-sk", "kubernetes", "terraform", "secrets", "kubeconfig")

				err = os.Setenv("KUBECONFIG", kcfgPath)
				Expect(err).To(Not(HaveOccurred()))

				DeferCleanup(func() {
					_ = os.Unsetenv("KUBECONFIG")
					_ = os.RemoveAll(w)
					pkillSession, err := KillOpenVPN()
					Expect(err).To(Not(HaveOccurred()))
					Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))
				})

				deleteClusterCmd := FuryctlDeleteCluster(furyctlYamlPath, distroPath, "", false, w)

				session, err := gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(session, 40*time.Minute).Should(gexec.Exit(0))
			})
		})
	})
)
