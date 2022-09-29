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

	_ = BeforeSuite(func() {
		tmpdir, err := os.MkdirTemp("", "furyctl-e2e")
		if err != nil {
			Fail(err.Error())
		}

		furyctl = filepath.Join(tmpdir, "furyctl")

		if out, err := exec.Command("go", "build", "-o", furyctl, "../../main.go").CombinedOutput(); err != nil {
			Fail(fmt.Sprintf("Could not build furyctl: %v\nOutput: %s", err, out))
		}
	})

	_ = Describe("furyctl", func() {
		Context("version", func() {
			It("should print its version information", func() {
				cmd := exec.Command(furyctl, "version")

				out, err := cmd.CombinedOutput()
				if err != nil {
					Fail(fmt.Sprintf("furyctl validate config failed: %v\nOutput: %s", err, out))
				}

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
				absBasepath, err := filepath.Abs(basepath)
				if err != nil {
					Fail(err.Error())
				}

				cmd := exec.Command(
					furyctl, "validate", "config",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath,
					"--debug",
				)

				return cmd.CombinedOutput()
			}

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
			FuryctlValidateDependencies := func(basepath string, binpath string) ([]byte, error) {
				absBasepath, err := filepath.Abs(basepath)
				if err != nil {
					Fail(err.Error())
				}

				cmd := exec.Command(
					furyctl, "validate", "dependencies",
					"--config", filepath.Join(absBasepath, "furyctl.yaml"),
					"--distro-location", absBasepath,
					"--bin-path", binpath,
					"--debug",
				)

				return cmd.CombinedOutput()
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
				os.Setenv("AWS_ACCESS_KEY_ID", "test")
				os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
				os.Setenv("AWS_DEFAULT_REGION", "test")

				defer func() {
					os.Unsetenv("AWS_ACCESS_KEY_ID")
					os.Unsetenv("AWS_SECRET_ACCESS_KEY")
					os.Unsetenv("AWS_DEFAULT_REGION")
				}()

				out, err := FuryctlValidateDependencies(
					"../data/e2e/validate/dependencies/correct",
					"../data/e2e/validate/dependencies/correct",
				)

				Expect(err).To(Not(HaveOccurred()))
				Expect(string(out)).To(ContainSubstring("Dependencies validation succeeded"))
			})
		})

		Context("dump template", func() {
			basepath := "../data/e2e/dump/template"
			FuryctlDumpTemplate := func(workdir string, dryRun bool) ([]byte, error) {
				args := []string{"dump", "template", "--debug", "--workdir", workdir}
				if dryRun {
					args = append(args, "--dry-run")
				}

				cmd := exec.Command(furyctl, args...)

				return cmd.CombinedOutput()
			}
			FileContent := func(path string) string {
				content, ferr := ioutil.ReadFile(path)
				if ferr != nil {
					Fail(ferr.Error())
				}

				return string(content)
			}

			It("no distribution file", func() {
				out, err := FuryctlDumpTemplate(basepath+"/no-distribution-yaml", false)

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("distribution.yaml: no such file or directory"))
			})

			It("no furyctl.yaml file", func() {
				out, err := FuryctlDumpTemplate(basepath+"/no-furyctl-yaml", false)

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("furyctl.yaml: no such file or directory"))
			})

			It("no data property in distribution.yaml file", func() {
				out, err := FuryctlDumpTemplate(basepath+"/distribution-yaml-no-data-property", false)

				Expect(err).To(HaveOccurred())
				Expect(string(out)).To(ContainSubstring("incorrect base file, cannot access key data on map"))
			})

			It("empty template", func() {
				_, err := FuryctlDumpTemplate(basepath+"/empty", false)

				Expect(err).To(HaveOccurred())
				Expect(basepath + "/empty/target/file.txt").To(Not(BeAnExistingFile()))
			})

			It("simple template dry-run", func() {
				_, err := FuryctlDumpTemplate(basepath+"/simple-dry-run", true)

				Expect(err).To(Not(HaveOccurred()))
				Expect(FileContent(basepath + "/simple-dry-run/target/file.txt")).To(ContainSubstring("testValue"))
			})

			It("simple template", func() {
				_, err := FuryctlDumpTemplate(basepath+"/simple", false)

				Expect(err).To(Not(HaveOccurred()))
				Expect(FileContent(basepath + "/simple/target/file.txt")).To(ContainSubstring("testValue"))
			})

			It("complex template dry-run", func() {
				_, err := FuryctlDumpTemplate(basepath+"/complex-dry-run", true)

				Expect(err).To(Not(HaveOccurred()))
				Expect(basepath + "/complex-dry-run/target/config/example.yaml").To(BeAnExistingFile())
				Expect(basepath + "/complex-dry-run/target/kustomization.yaml").To(BeAnExistingFile())
				Expect(FileContent(basepath + "/complex-dry-run/target/config/example.yaml")).
					To(ContainSubstring("configdata: example"))
				Expect(FileContent(basepath + "/complex-dry-run/target/kustomization.yaml")).
					To(Equal(FileContent(basepath + "/complex-dry-run/data/expected-kustomization.yaml")))
			})

			It("complex template", func() {
				_, err := FuryctlDumpTemplate(basepath+"/complex", false)

				Expect(err).To(Not(HaveOccurred()))
				Expect(basepath + "/complex/target/config/example.yaml").To(BeAnExistingFile())
				Expect(basepath + "/complex/target/kustomization.yaml").To(BeAnExistingFile())
				Expect(FileContent(basepath + "/complex/target/config/example.yaml")).
					To(ContainSubstring("configdata: example"))
				Expect(FileContent(basepath + "/complex/target/kustomization.yaml")).
					To(Equal(FileContent(basepath + "/complex/data/expected-kustomization.yaml")))
			})
		})
	})
)
