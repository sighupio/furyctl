// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package expensive_test

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
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

	FindFileStartingWith = func(pt, prefix string) (string, error) {
		files, err := os.ReadDir(pt)
		if err != nil {
			return "", err
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			if strings.HasPrefix(f.Name(), prefix) {
				return path.Join(pt, f.Name()), nil
			}
		}

		return "", fmt.Errorf("file not found in dir %s starting with name %s", pt, prefix)
	}

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

				//DeferCleanup(func() error {
				//	return os.RemoveAll(w)
				//})
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
				tfBinPath := path.Join(workDir, "vendor", "bin", "terraform")
				cmd := exec.Command(tfBinPath, "destroy", "-auto-approve")
				cmd.Dir = workDir

				return gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			}

			ClusterTeardown := func() error {
				kubeDestroy, err := TerraformDestroy(path.Join(w, ".kubernetes", "terraform"))
				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeDestroy, 15*time.Minute).Should(gexec.Exit(0))

				infraDestroy, err := TerraformDestroy(path.Join(w, ".infrastructure", "terraform"))
				Expect(err).To(Not(HaveOccurred()))
				Eventually(infraDestroy, 15*time.Minute).Should(gexec.Exit(0))

				pkillSession, err := KillOpenVPN()
				Expect(err).To(Not(HaveOccurred()))
				Eventually(pkillSession, 10*time.Second).Should(gexec.Exit(0))

				return nil
			}

			It("create cluster phase infrastructure on dry-run", func() {
				furyctlYamlPath := path.Join(absBasePath, "data/furyctl.yaml")
				distroPath := path.Join(absBasePath, "data")
				tfPath := path.Join(w, ".infrastructure", "terraform")

				createInfraCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "infrastructure", true)
				session, err := gexec.Start(createInfraCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))
				Expect(path.Join(tfPath, "plan")).To(BeADirectory())

				_, err = FindFileStartingWith(path.Join(tfPath, "plan"), "plan-")

				Expect(err).To(Not(HaveOccurred()))
				Expect(path.Join(tfPath, "plan", "terraform.plan")).To(BeAnExistingFile())
				Expect(session).To(gexec.Exit(0))
			})

			It("create cluster phase kubernetes on dry-run", func() {
				furyctlYamlPath := path.Join(absBasePath, "data/furyctl.yaml")
				distroPath := path.Join(absBasePath, "data")
				tfPath := path.Join(w, ".kubernetes", "terraform")

				createKubeCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "kubernetes", true)
				session, err := gexec.Start(createKubeCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))
				Expect(path.Join(tfPath, "plan")).To(BeADirectory())

				_, err = FindFileStartingWith(path.Join(tfPath, "plan"), "plan-")

				Expect(err).To(Not(HaveOccurred()))
				Expect(path.Join(tfPath, "plan", "terraform.plan")).To(BeAnExistingFile())
				Expect(session).To(gexec.Exit(0))
			})

			FIt("create a complete cluster", func() {
				furyctlYamlPath := path.Join(absBasePath, "data/furyctl.yaml")
				distroPath := path.Join(absBasePath, "data")
				kubeBinPath := path.Join(w, "vendor", "bin", "kubectl")
				kcfgPath := path.Join(w, ".kubernetes", "terraform", "secrets", "kubeconfig")

				defer ClusterTeardown()

				createClusterCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "", false)

				session, err := gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kcfgPath, 40*time.Minute).Should(BeAnExistingFile())
				Eventually(session, 20*time.Minute).Should(gexec.Exit(0))

				kubeCmd := exec.Command(kubeBinPath, "--kubeconfig", kcfgPath, "get", "nodes")

				kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).To(Not(HaveOccurred()))
				Eventually(kubeSession, 5*time.Minute).Should(gexec.Exit(0))
			})
		})
	})
)
