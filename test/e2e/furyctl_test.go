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
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	_ = BeforeSuite(func() {
		tmpdir := MkdirTemp("furyctl-e2e")

		furyctl = filepath.Join(tmpdir, "furyctl")

		if out, err := exec.Command("go", "build", "-o", furyctl, "../../main.go").CombinedOutput(); err != nil {
			Fail(fmt.Sprintf("Could not build furyctl: %v\nOutput: %s", err, out))
		}
	})

	_ = Describe("furyctl", func() {
		Context("version", func() {
			It("should print its version information", func() {
				out, err := exec.Command(furyctl, "version").CombinedOutput()

				Expect(err).To(Not(HaveOccurred()))
				Expect(string(out)).To(ContainSubstring(
					"buildTime: unknown\n" +
						"gitCommit: unknown\n" +
						"goVersion: unknown\n" +
						"osArch: unknown\n" +
						"version: unknown\n",
				))
			})
		})

		Context("validate config", func() {
			FuryctlValidateConfig := func(basepath string) ([]byte, error) {
				absBasepath := Abs(basepath)

				return exec.Command(
					furyctl, "validate", "config",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath,
					"--debug",
				).CombinedOutput()
			}

			It("should report an error when the furyctl.yaml is not found", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/")

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("furyctl.yaml: no such file or directory"))
			})

			It("should report an error when the kfd.yaml is not found", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/nodistro")

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("kfd.yaml: no such file or directory"))
			})

			It("should report an error when config validation fails", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/wrong")

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("config validation failed"))
			})

			It("should exit without errors when config validation succeeds", func() {
				out, err := FuryctlValidateConfig("../data/e2e/validate/config/correct")

				Expect(err).To(Not(HaveOccurred()))
				Expect(string(out)).To(ContainSubstring("config validation succeeded"))
			})
		})

		Context("validate dependencies", func() {
			FuryctlValidateDependencies := func(basepath, binpath string) ([]byte, error) {
				absBasepath := Abs(basepath)

				return exec.Command(
					furyctl, "validate", "dependencies",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath,
					"--bin-path", binpath,
					"--debug",
				).CombinedOutput()
			}

			It("should report an error when dependencies are missing", func() {
				out, err := FuryctlValidateDependencies("../data/e2e/validate/dependencies/missing", "/tmp")

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("ansible: no such file or directory"))
				Expect(string(out)).To(ContainSubstring("terraform: no such file or directory"))
				Expect(string(out)).To(ContainSubstring("kubectl: no such file or directory"))
				Expect(string(out)).To(ContainSubstring("kustomize: no such file or directory"))
				Expect(string(out)).To(ContainSubstring("furyagent: no such file or directory"))
				Expect(string(out)).To(ContainSubstring("missing environment variable: AWS_ACCESS_KEY_ID"))
				Expect(string(out)).To(ContainSubstring("missing environment variable: AWS_SECRET_ACCESS_KEY"))
				Expect(string(out)).To(ContainSubstring("missing environment variable: AWS_DEFAULT_REGION"))
			})

			It("should report an error when dependencies are wrong", func() {
				out, err := FuryctlValidateDependencies(
					"../data/e2e/validate/dependencies/wrong",
					"../data/e2e/validate/dependencies/wrong",
				)

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(
					ContainSubstring("ansible: wrong tool version - installed = 2.11.1, expected = 2.11.2"),
				)
				Expect(string(out)).To(
					ContainSubstring("furyagent: wrong tool version - installed = 0.2.4, expected = 0.3.0"),
				)
				Expect(string(out)).To(
					ContainSubstring("kubectl: wrong tool version - installed = 1.23.6, expected = 1.23.7"),
				)
				Expect(string(out)).To(
					ContainSubstring("kustomize: wrong tool version - installed = 3.9.0, expected = 3.10.0"),
				)
				Expect(string(out)).To(
					ContainSubstring("terraform: wrong tool version - installed = 0.15.3, expected = 0.15.4"),
				)
				Expect(string(out)).To(ContainSubstring("missing environment variable: AWS_ACCESS_KEY_ID"))
				Expect(string(out)).To(ContainSubstring("missing environment variable: AWS_SECRET_ACCESS_KEY"))
				Expect(string(out)).To(ContainSubstring("missing environment variable: AWS_DEFAULT_REGION"))
			})

			It("should exit without errors when dependencies are correct", func() {
				RestoreEnvVars := BackupEnvVars("AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION")
				defer RestoreEnvVars()

				os.Setenv("AWS_ACCESS_KEY_ID", "test")
				os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
				os.Setenv("AWS_DEFAULT_REGION", "test")

				out, err := FuryctlValidateDependencies(
					"../data/e2e/validate/dependencies/correct",
					"../data/e2e/validate/dependencies/correct",
				)

				Expect(err).To(Not(HaveOccurred()))
				Expect(string(out)).To(ContainSubstring("Dependencies validation succeeded"))
			})
		})

		Context("download dependencies", Label("slow"), func() {
			basepath := "../data/e2e/download/dependencies"
			FuryctlDownloadDependencies := func(basepath string) ([]byte, error) {
				absBasepath := Abs(basepath)

				return exec.Command(
					furyctl, "download", "dependencies",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath+"/distro",
					"--workdir", absBasepath,
					"--debug",
				).CombinedOutput()
			}

			It("should download all dependencies for v1.23.3", func() {
				bp := basepath + "/v1.23.3"
				vp := bp + "/vendor"

				RemoveAll(vp)
				defer RemoveAll(vp)

				_, err := FuryctlDownloadDependencies(bp)

				Expect(err).To(Not(HaveOccurred()))
				Expect(vp + "/bin/furyagent").To(BeAnExistingFile())
				Expect(vp + "/bin/kubectl").To(BeAnExistingFile())
				Expect(vp + "/bin/kustomize").To(BeAnExistingFile())
				Expect(vp + "/bin/terraform").To(BeAnExistingFile())
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
			FuryctlDumpTemplate := func(workdir string, dryRun bool) ([]byte, error) {
				args := []string{"dump", "template", "--debug", "--workdir", workdir}
				if dryRun {
					args = append(args, "--dry-run")
				}

				return exec.Command(furyctl, args...).CombinedOutput()
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
				Expect(string(out)).To(ContainSubstring("distribution.yaml: no such file or directory"))
			})

			It("fails if no furyctl.yaml file is found", func() {
				bp := Setup("no-furyctl-yaml")

				out, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("furyctl.yaml: no such file or directory"))
			})

			It("fails if no data properties are found in distribution.yaml file", func() {
				bp := Setup("distribution-yaml-no-data-property")

				out, err := FuryctlDumpTemplate(bp, false)

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("incorrect base file, cannot access key data on map"))
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
			FuryctlCreateConfig := func(workdir string) ([]byte, error) {
				return exec.Command(
					furyctl, "create", "config",
					"--config", workdir+"/target/furyctl.yaml",
					"--debug",
				).CombinedOutput()
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
	})
)
