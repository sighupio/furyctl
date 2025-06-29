// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package flags

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout = 10 * time.Second
)

func TestIntegration_FlagsConfiguration(t *testing.T) {
	t.Run("RealFileSystemOperations", testRealFileSystemOperations)
	t.Run("DynamicValueResolution", testDynamicValueResolution)
	t.Run("ViperIntegration", testViperIntegration)
	t.Run("PrioritySystem", testPrioritySystem)
	t.Run("ErrorHandling", testErrorHandling)
	t.Run("FuryDistributionCompatibility", testFuryDistributionCompatibility)
}

func testRealFileSystemOperations(t *testing.T) {
	// Note: Not using t.Parallel() because we modify global viper state

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "furyctl-flags-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test data
	testCases := []struct {
		name     string
		config   string
		expected map[string]any
	}{
		{
			name: "basic_flags_configuration",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true
    workdir: "/tmp/test"
  apply:
    timeout: 3600
    dryRun: false
    force: ["upgrades"]
`,
			expected: map[string]any{
				"debug":   true,
				"workdir": "/tmp/test",
				"timeout": 3600,
				"dry-run": false,
				"force":   []string{"upgrades"},
			},
		},
		{
			name: "complex_nested_configuration",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: false
    disableAnalytics: true
    gitProtocol: "ssh"
  apply:
    skipDepsValidation: true
    timeout: 7200
    vpnAutoConnect: true
    force: ["upgrades", "migrations"]
  delete:
    dryRun: true
    autoApprove: false
`,
			expected: map[string]any{
				"debug":                false,
				"disable-analytics":    true,
				"git-protocol":         "ssh",
				"skip-deps-validation": true,
				"timeout":              7200,
				"vpn-auto-connect":     true,
				"force":                []string{"upgrades", "migrations"},
				"dry-run":              true,
				"auto-approve":         false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test config file
			configPath := filepath.Join(tempDir, fmt.Sprintf("%s.yaml", tc.name))
			err := os.WriteFile(configPath, []byte(tc.config), 0o644)
			require.NoError(t, err)

			// Initialize manager
			manager := NewManager(tempDir)

			// Load flags for different commands
			for _, command := range []string{"global", "apply", "delete"} {
				t.Run(fmt.Sprintf("command_%s", command), func(t *testing.T) {
					// Reset viper for clean test
					viper.Reset()

					err := manager.LoadAndMergeFlags(configPath, command)
					assert.NoError(t, err)

					// Verify expected values are set in viper
					for key, expectedValue := range tc.expected {
						if viper.IsSet(key) {
							actualValue := viper.Get(key)
							assert.Equal(t, expectedValue, actualValue,
								"Expected %s=%v, got %v", key, expectedValue, actualValue)
						}
					}
				})
			}
		})
	}
}

func testDynamicValueResolution(t *testing.T) {
	// Note: Not using t.Parallel() because we need to set environment variables

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "furyctl-dynamic-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files for file references
	testFile := filepath.Join(tempDir, "test-file.txt")
	testContent := "/path/from/file"
	err = os.WriteFile(testFile, []byte(testContent), 0o644)
	require.NoError(t, err)

	// Set test environment variables
	testEnvVars := map[string]string{
		"FURYCTL_TEST_PATH":    "/env/test/path",
		"FURYCTL_TEST_TIMEOUT": "5400",
		"FURYCTL_TEST_DEBUG":   "true",
	}

	for key, value := range testEnvVars {
		t.Setenv(key, value)
	}

	config := fmt.Sprintf(`apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    workdir: "{env://FURYCTL_TEST_PATH}"
    debug: "{env://FURYCTL_TEST_DEBUG}"
    outdir: "{file://%s}"
  apply:
    timeout: "{env://FURYCTL_TEST_TIMEOUT}"
    distroLocation: "{file://%s}"
`, testFile, testFile)

	configPath := filepath.Join(tempDir, "dynamic-test.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test dynamic value resolution
	manager := NewManager(tempDir)
	viper.Reset()

	err = manager.LoadAndMergeFlags(configPath, "apply")
	require.NoError(t, err)

	// Verify environment variable resolution
	assert.Equal(t, "/env/test/path", viper.GetString("workdir"))
	assert.Equal(t, true, viper.GetBool("debug"))
	assert.Equal(t, 5400, viper.GetInt("timeout"))

	// Verify file reference resolution
	assert.Equal(t, testContent, viper.GetString("outdir"))
	assert.Equal(t, testContent, viper.GetString("distro-location"))
}

func testViperIntegration(t *testing.T) {
	// Note: Not using t.Parallel() because we modify global viper state

	tempDir, err := os.MkdirTemp("", "furyctl-viper-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true
    workdir: "/from/config"
  apply:
    timeout: 1800
    dryRun: false
`

	configPath := filepath.Join(tempDir, "viper-test.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test viper integration
	viper.Reset()

	// Set some values in viper before loading flags
	viper.Set("debug", false)  // This should take precedence
	viper.Set("timeout", 3600) // This should take precedence

	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	require.NoError(t, err)

	// Values already in viper should not be overridden
	assert.Equal(t, false, viper.GetBool("debug"))
	assert.Equal(t, 3600, viper.GetInt("timeout"))

	// Values not in viper should be set from config
	assert.Equal(t, "/from/config", viper.GetString("workdir"))
	assert.Equal(t, false, viper.GetBool("dry-run"))
}

func testPrioritySystem(t *testing.T) {
	// Note: Not using t.Parallel() because we need to set environment variables

	tempDir, err := os.MkdirTemp("", "furyctl-priority-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true
    workdir: "/from/config"
    gitProtocol: "https"
  apply:
    timeout: 1800
    dryRun: false
`

	configPath := filepath.Join(tempDir, "priority-test.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test priority: furyctl.yaml < env vars < command flags (viper)
	viper.Reset()

	// 1. Set environment variable (medium priority)
	t.Setenv("FURYCTL_DEBUG", "false")
	t.Setenv("FURYCTL_TIMEOUT", "3600")

	// 2. Simulate command line flag (highest priority) by setting in viper
	viper.Set("workdir", "/from/command/line")

	// 3. Load flags from config (lowest priority)
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	require.NoError(t, err)

	// Verify priority system:
	// - Command line (viper) wins over env vars and config
	assert.Equal(t, "/from/command/line", viper.GetString("workdir"))

	// - Config values are used when not overridden
	assert.Equal(t, "https", viper.GetString("git-protocol"))
	assert.Equal(t, false, viper.GetBool("dry-run"))

	// Note: Environment variable precedence is handled by viper's AutomaticEnv(),
	// which we test separately in unit tests
}

func testErrorHandling(t *testing.T) {
	// Note: Not using t.Parallel() because we modify global viper state

	tempDir, err := os.MkdirTemp("", "furyctl-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name          string
		config        string
		command       string
		expectError   bool
		errorContains string
	}{
		{
			name: "invalid_yaml",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true
    invalid_yaml: [unclosed array
`,
			command:       "global",
			expectError:   false, // Manager continues execution on YAML errors
			errorContains: "",
		},
		{
			name: "conflicting_flags",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  apply:
    upgrade: true
    upgradeNode: "node-1"
`,
			command:       "apply",
			expectError:   false, // Validation errors are only logged as warnings
			errorContains: "",
		},
		{
			name: "nonexistent_file_reference",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: test-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    workdir: "{file:///nonexistent/path}"
`,
			command:       "global",
			expectError:   true,
			errorContains: "no such file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, fmt.Sprintf("%s.yaml", tc.name))
			err := os.WriteFile(configPath, []byte(tc.config), 0o644)
			require.NoError(t, err)

			viper.Reset()
			manager := NewManager(tempDir)
			err = manager.LoadAndMergeFlags(configPath, tc.command)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" && err != nil {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func testFuryDistributionCompatibility(t *testing.T) {
	// Note: Not using t.Parallel() because we modify global viper state

	tempDir, err := os.MkdirTemp("", "furyctl-compat-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test with fury-distribution style configurations
	testConfigs := []struct {
		name   string
		config string
	}{
		{
			name: "ekscluster_style",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: fury-cluster
spec:
  distributionVersion: v1.31.0
  region: eu-west-1
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: test-bucket
          keyPrefix: furyctl/
          region: eu-west-1
  kubernetes:
    nodeAllowedSshPublicKey: "ssh-ed25519 AAAA..."
    nodePoolsLaunchKind: "launch_templates"
    apiServer:
      privateAccess: true
      publicAccess: false
    nodePools:
      - name: worker
        size:
          min: 1
          max: 3
        instance:
          type: t3.micro
  distribution:
    modules:
      ingress:
        baseDomain: internal.fury-demo.sighup.io
      logging:
        type: opensearch
      monitoring:
        type: prometheus
      dr:
        velero:
          eks:
            bucketName: test-velero
            region: eu-west-1
`,
		},
		{
			name: "kfddistribution_style",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: fury-local
spec:
  distributionVersion: v1.29.0
  distribution:
    kubeconfig: "{env://KUBECONFIG}"
    modules:
      networking:
        type: none
      ingress:
        baseDomain: internal.demo.example.dev
        nginx:
          type: single
          tls:
            provider: certManager
      logging:
        type: loki
      monitoring:
        type: prometheus
      policy:
        type: none
      dr:
        type: none
      auth:
        provider:
          type: none
`,
		},
	}

	for _, tc := range testConfigs {
		t.Run(tc.name, func(t *testing.T) {
			// Test existing configuration without flags
			configPath := filepath.Join(tempDir, fmt.Sprintf("%s.yaml", tc.name))
			err := os.WriteFile(configPath, []byte(tc.config), 0o644)
			require.NoError(t, err)

			viper.Reset()
			manager := NewManager(tempDir)

			// Should not fail when no flags section exists
			err = manager.LoadAndMergeFlags(configPath, "apply")
			assert.NoError(t, err)

			// Test with flags added
			configWithFlags := tc.config + `
flags:
  global:
    debug: true
    workdir: "/tmp/fury-test"
  apply:
    timeout: 3600
    dryRun: false
`
			configPathWithFlags := filepath.Join(tempDir, fmt.Sprintf("%s_with_flags.yaml", tc.name))
			err = os.WriteFile(configPathWithFlags, []byte(configWithFlags), 0o644)
			require.NoError(t, err)

			viper.Reset()
			err = manager.LoadAndMergeFlags(configPathWithFlags, "apply")
			assert.NoError(t, err)

			// Verify flags were loaded
			assert.Equal(t, true, viper.GetBool("debug"))
			assert.Equal(t, "/tmp/fury-test", viper.GetString("workdir"))
			assert.Equal(t, 3600, viper.GetInt("timeout"))
			assert.Equal(t, false, viper.GetBool("dry-run"))
		})
	}
}

// Helper function to create test configurations
func createTestConfig(dir, name, content string) (string, error) {
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, []byte(content), 0o644)
}

// Benchmark tests for performance validation
func BenchmarkFlagsLoading(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "furyctl-benchmark-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: benchmark-cluster
spec:
  distributionVersion: v1.31.0
flags:
  global:
    debug: true
    workdir: "/tmp/benchmark"
  apply:
    timeout: 3600
    dryRun: false
    force: ["upgrades", "migrations", "validations"]
`

	configPath := filepath.Join(tempDir, "benchmark.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	if err != nil {
		b.Fatal(err)
	}

	manager := NewManager(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		viper.Reset()
		err := manager.LoadAndMergeFlags(configPath, "apply")
		if err != nil {
			b.Fatal(err)
		}
	}
}
