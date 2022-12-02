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

	_ = BeforeSuite(func() {
		tmpdir, err := os.MkdirTemp("", "furyctl-expensive")
		Expect(err).To(Not(HaveOccurred()))

		furyctl = filepath.Join(tmpdir, "furyctl")

		cmd := exec.Command("go", "build", "-o", furyctl, "../../main.go")

		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).To(Not(HaveOccurred()))
		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
	})

	_ = Describe("furyctl", func() {
		Context("create cluster", func() {
			var w string
			var absBasePath string

			basepath := "../data/expensive/create/cluster"

			BeforeEach(func() {
				var err error

				absBasePath, err = filepath.Abs(basepath)
				Expect(err).To(Not(HaveOccurred()))

				w, err = os.MkdirTemp("", "create-cluster-test-")
				Expect(err).To(Not(HaveOccurred()))

				Expect(w).To(BeADirectory())

				DeferCleanup(func() error {
					return os.RemoveAll(w)
				})
			})

			FuryctlCreateCluster := func(cfgPath, distroPath, phase string, dryRun bool) *exec.Cmd {
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
				} else {
					args = append(args, "--vpn-auto-connect")
				}

				if dryRun {
					args = append(args, "--dry-run")
				}

				return exec.Command(furyctl, args...)
			}

			KillOpenVPN := func() (*gexec.Session, error) {
				cmd := exec.Command("sudo", "pkill", "openvpn")

				return gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			}

			TerraformDestroy := func(workDir string) (*gexec.Session, error) {
				tfBinPath := path.Join(w, "vendor", "bin", "terraform")
				cmd := exec.Command(tfBinPath, "destroy", "-auto-approve")
				cmd.Dir = workDir

				return gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			}

			ClusterTeardown := func() error {
				kubeDestroy, err := TerraformDestroy(path.Join(w, "kubernetes", "terraform"))
				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeDestroy, 10*time.Minute).Should(gexec.Exit(0))

				infraDestroy, err := TerraformDestroy(path.Join(w, "infrastructure", "terraform"))
				Expect(err).To(Not(HaveOccurred()))
				Eventually(infraDestroy, 10*time.Minute).Should(gexec.Exit(0))

				pkillSession, err := KillOpenVPN()
				Expect(err).To(Not(HaveOccurred()))
				Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))

				return nil
			}

			It("create a complete cluster", func() {
				furyctlYamlPath := path.Join(absBasePath, "data/furyctl.yaml")
				distroPath := path.Join(absBasePath, "data")
				kubeBinPath := path.Join(w, "vendor", "bin", "kubectl")
				tfInfraPath := path.Join(w, "infrastructure", "terraform")
				kcfgPath := path.Join(w, "kubernetes", "terraform", "secrets", "kubeconfig")

				defer ClusterTeardown()

				createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "", false)

				session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))
				Consistently(session, 3*time.Minute).ShouldNot(gexec.Exit())
				Expect(path.Join(tfInfraPath, "plan", "terraform.plan")).To(BeAnExistingFile())
				Eventually(kcfgPath, 30*time.Minute).Should(BeAnExistingFile())
				Eventually(session, 1*time.Minute).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubeBinPath, "--kubeconfig", kcfgPath, "get", "nodes")

				kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeSession, 2*time.Minute).Should(gexec.Exit(0))
			})
		})
	})
)
