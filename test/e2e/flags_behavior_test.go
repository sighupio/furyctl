//go:build flags_e2e

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestFlagsBehaviorE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flags Behavior E2E Suite")
}

var furyctlPath string

var _ = BeforeSuite(func() {
	// Build furyctl binary for testing
	var err error
	furyctlPath, err = gexec.Build("github.com/sighupio/furyctl")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = Describe("Flag Behavior E2E", func() {
	Context("Flag Loading Effects - No Distribution Required", func() {
		It("loads debug flag from config and shows early debug output", func() {
			// Use testdata config with debug: true
			configPath := "./testdata/test-global-flags.yaml"

			// Run furyctl validate config with config file (this command has --config flag)
			cmd := exec.Command(furyctlPath, "validate", "config", "--config", configPath, "--distro-location", "/tmp/fake-distro")

			// Capture both stdout and stderr since debug messages go to stderr
			output, _ := cmd.CombinedOutput()

			// Command may fail due to distribution not being available, but flags should still load
			outputStr := string(output)

			// Should show debug output if the flag was loaded
			Expect(outputStr).To(ContainSubstring("DEBU"), "Should show debug output when debug flag is loaded from config")
		})

		It("CLI debug flag overrides config", func() {
			// Create a temporary config with debug: false
			tempDir := GinkgoT().TempDir()
			configPath := filepath.Join(tempDir, "test-no-debug.yaml")

			configContent := `flags:
  global:
    debug: false
    log: "/tmp/no-debug-test.log"
`

			err := os.WriteFile(configPath, []byte(configContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			// Run with --debug to override config debug: false
			cmd := exec.Command(furyctlPath, "validate", "config", "--debug", "--config", configPath, "--distro-location", "/tmp/fake-distro")

			output, _ := cmd.CombinedOutput()

			outputStr := string(output)

			// Should show debug output even though config has debug: false (CLI override)
			Expect(outputStr).To(ContainSubstring("DEBU"), "Should show debug output with CLI debug override")
		})

		It("expands environment variables in early logging", func() {
			// Use testdata config with environment variable expansion
			configPath := "./testdata/test-env-valid.yaml"

			// Set PWD to a known value for the test
			testPWD := GinkgoT().TempDir()

			cmd := exec.Command(furyctlPath, "validate", "config", "--config", configPath, "--distro-location", "/tmp/fake-distro")
			cmd.Env = append(os.Environ(), "PWD="+testPWD)

			output, _ := cmd.CombinedOutput()

			outputStr := string(output)

			// Should succeed with env expansion (even if validation fails due to missing distro)
			// Check that PWD was expanded in the log path by looking for debug output
			if strings.Contains(outputStr, "DEBU") {
				Expect(outputStr).To(ContainSubstring(testPWD), "Debug output should contain expanded PWD path")
			}

			// The log file should be created with the expanded path
			expectedLogPath := filepath.Join(testPWD, "furyctl.log")

			// Give it a moment for any delayed file operations
			Eventually(func() bool {
				_, err := os.Stat(expectedLogPath)
				return err == nil
			}).Should(BeTrue(), "Log file should be created at expanded path: %s", expectedLogPath)
		})

		It("handles expansion errors gracefully", func() {
			// Use testdata config with invalid environment variable reference
			configPath := "./testdata/test-env-error.yaml"

			// Ensure NONEXISTENT_VAR is not set
			cmd := exec.Command(furyctlPath, "validate", "config", "--config", configPath, "--distro-location", "/tmp/fake-distro")
			cmd.Env = filterEnv(os.Environ(), "NONEXISTENT_VAR")

			output, err := cmd.CombinedOutput()

			// Command should fail gracefully due to missing env var
			Expect(err).To(HaveOccurred(), "furyctl should fail with missing env var")

			outputStr := string(output)

			// Should contain clear error message about the missing variable
			Expect(outputStr).To(ContainSubstring("NONEXISTENT_VAR"), "Error should mention the missing env var")

			// Should not crash or produce cryptic errors
			Expect(outputStr).NotTo(ContainSubstring("panic:"), "Should not panic")
			Expect(outputStr).NotTo(ContainSubstring("runtime error:"), "Should not have runtime error")
		})

		It("loads flags even when validation fails", func() {
			// This test verifies that flag loading works even when command fails
			// simulating CI environment behavior

			// Create minimal config with flags
			tempDir := GinkgoT().TempDir()
			configPath := filepath.Join(tempDir, "minimal-flags.yaml")

			configContent := `flags:
  global:
    debug: true
    log: "{env://HOME}/ci-test.log"
`

			err := os.WriteFile(configPath, []byte(configContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			// Run validate config which should load flags even if distro fails
			cmd := exec.Command(furyctlPath, "validate", "config", "--config", configPath, "--distro-location", "/tmp/fake-distro")

			output, _ := cmd.CombinedOutput()

			outputStr := string(output)

			// Should show debug output indicating flags were loaded
			// even if the command fails due to missing distribution
			Expect(outputStr).To(ContainSubstring("DEBU"), "Should show debug output indicating flag loading worked")
		})

		It("preserves flag precedence order", func() {
			// Create config with specific values
			tempDir := GinkgoT().TempDir()
			configPath := filepath.Join(tempDir, "precedence-test.yaml")

			configContent := `flags:
  global:
    debug: false
    log: "/tmp/config-precedence.log"
`

			err := os.WriteFile(configPath, []byte(configContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			// Test 1: Config values only (debug: false)
			cmd1 := exec.Command(furyctlPath, "validate", "config", "--config", configPath, "--distro-location", "/tmp/fake-distro")
			output1, _ := cmd1.CombinedOutput()

			outputStr1 := string(output1)
			// Should NOT show debug output when debug: false in config
			Expect(outputStr1).NotTo(ContainSubstring("DEBU"), "Should not show debug output when debug is false in config")

			// Test 2: CLI flag overrides config (--debug overrides config debug: false)
			cmd2 := exec.Command(furyctlPath, "validate", "config", "--debug", "--config", configPath, "--distro-location", "/tmp/fake-distro")
			output2, _ := cmd2.CombinedOutput()

			outputStr2 := string(output2)
			// Should show debug output when CLI --debug overrides config debug: false
			Expect(outputStr2).To(ContainSubstring("DEBU"), "Should show debug output when CLI --debug overrides config")
		})

		It("prevents directory creation with unexpanded dynamic values", func() {
			// REGRESSION TEST: This test ensures the critical fix that prevents
			// creating literal directories like "{env://NONEXISTENT_VAR}/"

			// Use testdata config with invalid environment variable reference
			configPath := "./testdata/test-env-error.yaml"

			// Get current working directory to check for unwanted directory creation
			workDir, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			// Ensure NONEXISTENT_VAR is not set
			cmd := exec.Command(furyctlPath, "validate", "config", "--config", configPath, "--distro-location", "/tmp/fake-distro")
			cmd.Env = filterEnv(os.Environ(), "NONEXISTENT_VAR")

			output, err := cmd.CombinedOutput()

			// Command should fail due to missing env var (this is the correct behavior)
			Expect(err).To(HaveOccurred(), "furyctl should fail with missing env var")

			outputStr := string(output)

			// Should contain clear error message about unexpanded dynamic values
			Expect(outputStr).To(ContainSubstring("cannot create log file with unexpanded dynamic values"), "Should mention unexpanded dynamic values error")
			Expect(outputStr).To(ContainSubstring("NONEXISTENT_VAR"), "Error should mention the missing env var")

			// CRITICAL: Verify no literal directory with unexpanded dynamic values was created
			badDirPath := filepath.Join(workDir, "{env:")
			_, err = os.Stat(badDirPath)
			Expect(err).To(HaveOccurred(), "Should NOT create literal directory with unexpanded dynamic values")

			// Also check common variations that could be created
			badPaths := []string{
				"{env://NONEXISTENT_VAR}",
				"{env://NONEXISTENT_VAR}/",
				filepath.Join(workDir, "{env://NONEXISTENT_VAR}"),
				filepath.Join(workDir, "{env://NONEXISTENT_VAR}"),
			}

			for _, badPath := range badPaths {
				_, err = os.Stat(badPath)
				Expect(err).To(HaveOccurred(), "Should NOT create directory: %s", badPath)
			}
		})

		It("works with different flag combinations", func() {
			// Test various flag loading scenarios using testdata files
			scenarios := []struct {
				name        string
				configFile  string
				extraArgs   []string
				envVars     []string
				expectDebug bool
			}{
				{
					name:        "global flags with debug true",
					configFile:  "./testdata/test-global-flags.yaml",
					extraArgs:   []string{},
					expectDebug: true,
				},
				{
					name:        "env expansion with debug",
					configFile:  "./testdata/test-env-valid.yaml",
					extraArgs:   []string{},
					envVars:     []string{"PWD=" + GinkgoT().TempDir()},
					expectDebug: true, // test-env-valid.yaml has debug: true
				},
				{
					name:        "CLI flag override to false",
					configFile:  "./testdata/test-global-flags.yaml",
					extraArgs:   []string{"--debug=false"},
					expectDebug: false,
				},
			}

			for _, scenario := range scenarios {
				By("Testing scenario: " + scenario.name)

				args := append([]string{"validate", "config", "--config", scenario.configFile, "--distro-location", "/tmp/fake-distro"}, scenario.extraArgs...)
				cmd := exec.Command(furyctlPath, args...)

				// Set custom environment variables if specified
				if len(scenario.envVars) > 0 {
					cmd.Env = append(os.Environ(), scenario.envVars...)
				}

				output, _ := cmd.CombinedOutput()
				outputStr := string(output)

				// Check if debug output matches expectation
				if scenario.expectDebug {
					Expect(outputStr).To(ContainSubstring("DEBU"), "Should show debug output for: %s", scenario.name)
				} else {
					Expect(outputStr).NotTo(ContainSubstring("DEBU"), "Should not show debug output for: %s", scenario.name)
				}
			}
		})
	})
})

// Helper function to filter out specific environment variables
func filterEnv(env []string, varToRemove string) []string {
	filtered := make([]string, 0, len(env))
	prefix := varToRemove + "="

	for _, e := range env {
		if !strings.HasPrefix(e, prefix) && e != varToRemove {
			filtered = append(filtered, e)
		}
	}

	return filtered
}
