// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build e2e

package e2e_test

import (
	"fmt"
	"io/ioutil"
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

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl E2e Suite")
}

var (
	furyctl string

	Abs = func(path string) string {
		absPath, err := filepath.Abs(path)
		if err != nil {
			Fail(err.Error())
		}

		return absPath
	}

	FileContent = func(path string) string {
		content, ferr := ioutil.ReadFile(path)
		if ferr != nil {
			Fail(ferr.Error())
		}

		return string(content)
	}

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

	MkdirTemp = func(pattern string) string {
		tmpdir, err := os.MkdirTemp("", pattern)
		if err != nil {
			Fail(err.Error())
		}

		return tmpdir
	}

	RemoveAll = func(path string) {
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			Fail(err.Error())
		}
	}

	BackupEnvVars = func(vars ...string) func() {
		backup := make(map[string]string)
		remove := make([]string, 0)

		for _, v := range vars {
			if val, ok := os.LookupEnv(v); ok {
				backup[v] = val
			} else {
				remove = append(remove, v)
			}
		}

		return func() {
			for k, v := range backup {
				os.Setenv(k, v)
			}

			for _, v := range remove {
				os.Unsetenv(v)
			}
		}
	}

	RunCmd = func(cmd string, args ...string) (string, error) {
		out, err := exec.Command(cmd, args...).CombinedOutput()

		return string(out), err
	}

	_ = BeforeSuite(func() {
		tmpdir := MkdirTemp("furyctl-e2e")

		furyctl = filepath.Join(tmpdir, "furyctl")

		if out, err := RunCmd("go", "build", "-o", furyctl, "../../main.go"); err != nil {
			Fail(fmt.Sprintf("Could not build furyctl: %v\nOutput: %s", err, out))
		}
	})

	_ = Describe("furyctl", func() {
		Context("version", func() {
			It("should print its version information", func() {
				out, err := RunCmd(furyctl, "version", "--disable-analytics", "true")

				Expect(err).To(Not(HaveOccurred()))
				Expect(out).To(ContainSubstring(
					"buildTime: unknown\n" +
						"gitCommit: unknown\n" +
						"goVersion: unknown\n" +
						"osArch: unknown\n" +
						"version: unknown\n",
				))
			})
		})

		Context("validate config", func() {
			FuryctlValidateConfig := func(basepath string) (string, error) {
				absBasepath := Abs(basepath)

				return RunCmd(
					furyctl, "validate", "config",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath,
					"--debug",
					"--disable-analytics", "true",
				)
			}

			It("should report an error when the furyctl.yaml is not found", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/")

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("furyctl.yaml: no such file or directory"))
			})

			It("should report an error when the kfd.yaml is not found", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/nodistro")

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("kfd.yaml: no such file or directory"))
			})

			It("should report an error when config validation fails", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/wrong")

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("config validation failed"))
			})

			It("should exit without errors when config validation succeeds", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/correct")

				Expect(err).To(Not(HaveOccurred()))
				Expect(out).To(ContainSubstring("config validation succeeded"))
			})
		})

		Context("validate dependencies", func() {
			FuryctlValidateDependencies := func(basepath, binpath string) (string, error) {
				absBasepath := Abs(basepath)

				return RunCmd(
					furyctl, "validate", "dependencies",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath,
					"--bin-path", binpath,
					"--debug",
					"--disable-analytics", "true",
				)
			}

			It("should report an error when dependencies are missing", func() {
				RestoreEnvVars := BackupEnvVars("PATH", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION")
				defer RestoreEnvVars()

				os.Unsetenv("AWS_ACCESS_KEY_ID")
				os.Unsetenv("AWS_SECRET_ACCESS_KEY")
				os.Unsetenv("AWS_DEFAULT_REGION")

				out, err := FuryctlValidateDependencies("../data/e2e/validate/dependencies/missing", "/tmp")

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("ansible:"))
				Expect(out).To(ContainSubstring("terraform:"))
				Expect(out).To(ContainSubstring("kubectl:"))
				Expect(out).To(ContainSubstring("kustomize:"))
				Expect(out).To(ContainSubstring("furyagent:"))
				Expect(out).To(ContainSubstring("missing environment variable: AWS_ACCESS_KEY_ID"))
				Expect(out).To(ContainSubstring("missing environment variable: AWS_SECRET_ACCESS_KEY"))
				Expect(out).To(ContainSubstring("missing environment variable: AWS_DEFAULT_REGION"))
			})

			It("should report an error when dependencies are wrong", Serial, func() {
				RestoreEnvVars := BackupEnvVars("PATH", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION")
				defer RestoreEnvVars()

				bp := Abs("../data/e2e/validate/dependencies/wrong")

				os.Setenv("PATH", bp+":"+os.Getenv("PATH"))
				os.Unsetenv("AWS_ACCESS_KEY_ID")
				os.Unsetenv("AWS_SECRET_ACCESS_KEY")
				os.Unsetenv("AWS_DEFAULT_REGION")

				out, err := FuryctlValidateDependencies(bp, bp)

				Expect(err).To(HaveOccurred())
				Expect(out).To(
					ContainSubstring("ansible: wrong tool version - installed = 2.11.1, expected = 2.11.2"),
				)
				Expect(out).To(
					ContainSubstring("furyagent: wrong tool version - installed = 0.2.4, expected = 0.3.0"),
				)
				Expect(out).To(
					ContainSubstring("kubectl: wrong tool version - installed = 1.23.6, expected = 1.23.7"),
				)
				Expect(out).To(
					ContainSubstring("kustomize: wrong tool version - installed = 3.9.0, expected = 3.10.0"),
				)
				Expect(out).To(
					ContainSubstring("terraform: wrong tool version - installed = 0.15.3, expected = 0.15.4"),
				)
				Expect(out).To(ContainSubstring("missing environment variable: AWS_ACCESS_KEY_ID"))
				Expect(out).To(ContainSubstring("missing environment variable: AWS_SECRET_ACCESS_KEY"))
				Expect(out).To(ContainSubstring("missing environment variable: AWS_DEFAULT_REGION"))
			})

			It("should exit without errors when dependencies are correct", Serial, func() {
				RestoreEnvVars := BackupEnvVars("PATH", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION")
				defer RestoreEnvVars()

				bp := Abs("../data/e2e/validate/dependencies/correct")

				os.Setenv("PATH", bp+":"+os.Getenv("PATH"))
				os.Setenv("AWS_ACCESS_KEY_ID", "test")
				os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
				os.Setenv("AWS_DEFAULT_REGION", "test")

				out, err := FuryctlValidateDependencies(bp, bp)

				Expect(err).To(Not(HaveOccurred()))
				Expect(out).To(ContainSubstring("Dependencies validation succeeded"))
			})
		})

		Context("download dependencies", Label("slow"), func() {
			basepath := "../data/e2e/download/dependencies"
			FuryctlDownloadDependencies := func(basepath string) (string, error) {
				absBasepath := Abs(basepath)

				return RunCmd(
					furyctl, "download", "dependencies",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath+"/distro",
					"--workdir", absBasepath,
					"--debug",
					"--disable-analytics", "true",
				)
			}

			It("should download all dependencies for v1.23.3", func() {
				bp := basepath + "/v1.23.3"

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				vp := path.Join(homeDir, ".furyctl", "awesome-cluster-staging", "vendor")
				binP := path.Join(homeDir, ".furyctl", "bin")

				RemoveAll(vp)
				defer RemoveAll(vp)

				_, err = FuryctlDownloadDependencies(bp)

				Expect(err).To(Not(HaveOccurred()))
				Expect(binP + "/furyagent/0.3.0/furyagent").To(BeAnExistingFile())
				Expect(binP + "/kubectl/1.23.10/kubectl").To(BeAnExistingFile())
				Expect(binP + "/kustomize/3.10.0/kustomize").To(BeAnExistingFile())
				Expect(binP + "/terraform/0.15.4/terraform").To(BeAnExistingFile())
				Expect(vp + "/installers/eks/README.md").To(BeAnExistingFile())
				Expect(vp + "/installers/eks/modules/eks/main.tf").To(BeAnExistingFile())
				Expect(vp + "/installers/eks/modules/vpc-and-vpn/main.tf").To(BeAnExistingFile())
				Expect(vp + "/modules/auth/README.md").To(BeAnExistingFile())
				Expect(vp + "/modules/auth/katalog/gangway/kustomization.yaml").To(BeAnExistingFile())
				Expect(vp + "/modules/dr/README.md").To(BeAnExistingFile())
				Expect(vp + "/modules/dr/katalog/velero/velero-aws/kustomization.yaml").To(BeAnExistingFile())
				Expect(vp + "/modules/ingress/README.md").To(BeAnExistingFile())
				Expect(vp + "/modules/ingress/katalog/nginx/kustomization.yaml").To(BeAnExistingFile())
				Expect(vp + "/modules/logging/README.md").To(BeAnExistingFile())
				Expect(vp + "/modules/logging/katalog/configs/kustomization.yaml").To(BeAnExistingFile())
				Expect(vp + "/modules/monitoring/README.md").To(BeAnExistingFile())
				Expect(vp + "/modules/monitoring/katalog/configs/kustomization.yaml").To(BeAnExistingFile())
				Expect(vp + "/modules/opa/README.md").To(BeAnExistingFile())
				Expect(vp + "/modules/opa/katalog/gatekeeper/kustomization.yaml").To(BeAnExistingFile())
			})
		})

		Context("dump template", func() {
			basepath := "../data/e2e/dump/template"
			FuryctlDumpTemplate := func(workdir string, dryRun bool) (string, error) {
				args := []string{"dump", "template", "--debug", "--workdir", workdir, "--disable-analytics", "true"}
				if dryRun {
					args = append(args, "--dry-run")
				}

				return RunCmd(furyctl, args...)
			}
			Setup := func(folder string) string {
				bp := filepath.Join(basepath, folder)
				tp := filepath.Join(bp, "target")

				RemoveAll(tp)

				return bp
			}

			It("fails if no distribution yaml is found", func() {
				bp := Setup("no-distribution-yaml")

				out, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("distribution.yaml: no such file or directory"))
			})

			It("fails if no furyctl.yaml file is found", func() {
				bp := Setup("no-furyctl-yaml")

				out, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("furyctl.yaml: no such file or directory"))
			})

			It("fails if no data properties are found in distribution.yaml file", func() {
				bp := Setup("distribution-yaml-no-data-property")

				out, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("incorrect base file, cannot access key data on map"))
			})

			It("fails if given an empty template", func() {
				bp := Setup("empty")

				_, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(HaveOccurred())
				Expect(bp + "/target/file.txt").To(Not(BeAnExistingFile()))
			})

			It("succeeds when given a simple template on dry-run", func() {
				bp := Setup("simple-dry-run")

				_, err := FuryctlDumpTemplate(bp, true)

				Expect(err).To(Not(HaveOccurred()))
				Expect(FileContent(bp + "/target/file.txt")).To(ContainSubstring("testValue"))
			})

			It("succeeds when given a simple template", func() {
				bp := Setup("simple")

				_, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(Not(HaveOccurred()))
				Expect(FileContent(bp + "/target/file.txt")).To(ContainSubstring("testValue"))
			})

			It("succeeds when given a complex template on dry-run", func() {
				bp := Setup("complex-dry-run")

				_, err := FuryctlDumpTemplate(bp, true)

				Expect(err).To(Not(HaveOccurred()))
				Expect(bp + "/target/config/example.yaml").To(BeAnExistingFile())
				Expect(bp + "/target/kustomization.yaml").To(BeAnExistingFile())
				Expect(FileContent(bp + "/target/config/example.yaml")).To(ContainSubstring("configdata: example"))
				Expect(FileContent(bp + "/target/kustomization.yaml")).
					To(Equal(FileContent(bp + "/data/expected-kustomization.yaml")))
			})

			It("succeeds when given a complex template", func() {
				bp := Setup("complex")

				_, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(Not(HaveOccurred()))
				Expect(bp + "/target/config/example.yaml").To(BeAnExistingFile())
				Expect(bp + "/target/kustomization.yaml").To(BeAnExistingFile())
				Expect(FileContent(bp + "/target/config/example.yaml")).To(ContainSubstring("configdata: example"))
				Expect(FileContent(bp + "/target/kustomization.yaml")).
					To(Equal(FileContent(bp + "/data/expected-kustomization.yaml")))
			})
		})

		Context("create config", func() {
			basepath := "../data/e2e/create/config"
			FuryctlCreateConfig := func(workdir string) (string, error) {
				absBasepath := Abs(basepath)

				return RunCmd(
					furyctl, "create", "config",
					"--config", workdir+"/target/furyctl.yaml",
					"--debug",
					"--disable-analytics", "true",
					"--distro-location", absBasepath+"/distro",
				)
			}
			Setup := func(folder string) string {
				bp := filepath.Join(basepath, folder)
				tp := filepath.Join(bp, "target")

				RemoveAll(tp)

				return bp
			}

			It("scaffolds a new furyctl.yaml file", func() {
				bp := Setup("default")

				_, err := FuryctlCreateConfig(bp)

				Expect(err).To(Not(HaveOccurred()))
				Expect(bp + "/target/furyctl.yaml").To(BeAnExistingFile())
				Expect(FileContent(bp + "/target/furyctl.yaml")).
					To(Equal(FileContent(bp + "/data/expected-furyctl.yaml")))
			})
		})

		Context("create cluster dry run", Ordered, Serial, Label("slow"), func() {
			var w string
			var absBasePath string

			basepath := "../data/e2e/create/cluster"

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
					"--disable-analytics",
					"true",
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

			It("create cluster phase infrastructure on dry-run", func() {
				RestoreEnvVars := BackupEnvVars("PATH")
				defer RestoreEnvVars()

				bp := Abs("../data/e2e/create/cluster/bin_mock")

				err := os.Setenv("PATH", bp+":"+os.Getenv("PATH"))
				Expect(err).To(Not(HaveOccurred()))

				furyctlYamlPath := path.Join(absBasePath, "data/furyctl.yaml")
				distroPath := path.Join(absBasePath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				tfPath := path.Join(homeDir, ".furyctl", "furyctl-dev-aws", "infrastructure", "terraform")

				createInfraCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "infrastructure", true)
				session, err := gexec.Start(createInfraCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(path.Join(tfPath, "plan", "terraform.plan"), 120*time.Second).Should(BeAnExistingFile())

				Eventually(session).Should(gexec.Exit(0))

				_, err = FindFileStartingWith(path.Join(tfPath, "plan"), "plan-")
				Expect(err).To(Not(HaveOccurred()))
			})

			It("create cluster phase kubernetes on dry-run", func() {
				RestoreEnvVars := BackupEnvVars("PATH")
				defer RestoreEnvVars()

				bp := Abs("../data/e2e/create/cluster/bin_mock")

				err := os.Setenv("PATH", bp+":"+os.Getenv("PATH"))
				Expect(err).To(Not(HaveOccurred()))

				furyctlYamlPath := path.Join(absBasePath, "data/furyctl.yaml")
				distroPath := path.Join(absBasePath, "data")

				homeDir, err := os.UserHomeDir()
				Expect(err).To(Not(HaveOccurred()))

				tfPath := path.Join(homeDir, ".furyctl", "furyctl-dev-aws", "kubernetes", "terraform")

				createKubeCmd := FuryctlCreateCluster(furyctlYamlPath, distroPath, "kubernetes", true)
				session, err := gexec.Start(createKubeCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).To(Not(HaveOccurred()))

				Eventually(path.Join(tfPath, "plan", "terraform.plan"), 120*time.Second).Should(BeAnExistingFile())

				Eventually(session).Should(gexec.Exit(0))

				_, err = FindFileStartingWith(path.Join(tfPath, "plan"), "plan-")
				Expect(err).To(Not(HaveOccurred()))
			})
		})
	})
)
