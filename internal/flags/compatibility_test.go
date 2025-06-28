package flags

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuryDistributionCompatibility(t *testing.T) {
	t.Run("EKSCluster_ExistingConfig", testEKSClusterExistingConfig)
	t.Run("EKSCluster_WithFlags", testEKSClusterWithFlags)
	t.Run("KFDDistribution_ExistingConfig", testKFDDistributionExistingConfig)
	t.Run("KFDDistribution_WithFlags", testKFDDistributionWithFlags)
	t.Run("OnPremises_ExistingConfig", testOnPremisesExistingConfig)
	t.Run("OnPremises_WithFlags", testOnPremisesWithFlags)
}

func testEKSClusterExistingConfig(t *testing.T) {
	// Based on fury-distribution/test/data/e2e/create/cluster/infrastructure/data/furyctl.yaml
	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: furyctl-dev-aws
spec:
  distributionVersion: "v1.25.4"
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: TERRAFORM_TF_STATES_BUCKET_NAME
          keyPrefix: furyctl-next-create-cluster/
          region: eu-west-1
  region: eu-west-1
  tags:
    env: "test"
    k8s: "awesome"
  infrastructure:
    vpc:
      network:
        cidr: 10.0.0.0/16
        subnetsCidrs:
          private:
            - 10.0.182.0/24
            - 10.0.172.0/24
            - 10.0.162.0/24
          public:
            - 10.0.20.0/24
            - 10.0.30.0/24
            - 10.0.40.0/24
    vpn:
      instances: 1
      port: 1194
      instanceType: t3.micro
      diskSize: 50
      operatorName: sighup
      dhParamsBits: 2048
      vpnClientsSubnetCidr: 192.168.200.0/24
      ssh:
        publicKeys:
          - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAQt/UN/edbCpeWU6M17UqCUqTXs96b7DDWUcbdBrATP"
        githubUsersName:
          - Al-Pragliola
        allowedFromCidrs:
          - 0.0.0.0/0
  kubernetes:
    nodeAllowedSshPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAQt/UN/edbCpeWU6M17UqCUqTXs96b7DDWUcbdBrATP"
    nodePoolsLaunchKind: "launch_templates"
    apiServer:
      privateAccess: true
      publicAccess: false
      privateAccessCidrs: [ '0.0.0.0/0' ]
      publicAccessCidrs: [ '0.0.0.0/0' ]
    nodePools:
      - name: worker
        size:
          min: 1
          max: 3
        instance:
          type: t3.micro
        labels:
          nodepool: worker
          node.kubernetes.io/role: worker
        taints:
          - node.kubernetes.io/role=worker:NoSchedule
        tags:
          k8s.io/cluster-autoscaler/node-template/label/nodepool: "worker"
          k8s.io/cluster-autoscaler/node-template/label/node.kubernetes.io/role: "worker"
  distribution:
    modules:
      ingress:
        baseDomain: internal.fury-demo.sighup.io
        nginx:
          type: single
          tls:
            provider: certManager
        certManager:
          clusterIssuer:
            name: letsencrypt-fury
            email: engineering+fury-distribution@sighup.io
            type: http01
        dns:
          public:
            name: "fury-demo.sighup.io"
            create: false
          private:
            create: true
            name: "internal.fury-demo.sighup.io"
      logging:
        overrides:
          nodeSelector: {}
          tolerations: []
        opensearch:
          type: single
          resources:
            requests:
              cpu: ""
              memory: ""
            limits:
              cpu: ""
              memory: ""
          storageSize: "150Gi"
      monitoring:
        overrides:
          nodeSelector: {}
          tolerations: []
        prometheus:
          resources:
            requests:
              cpu: ""
              memory: ""
            limits:
              cpu: ""
              memory: ""
      policy:
        overrides:
          nodeSelector: {}
          tolerations: []
        gatekeeper:
          additionalExcludedNamespaces: []
      dr:
        velero:
          eks:
            bucketName: example-velero
            region: eu-west-1
      auth:
        provider:
          type: none
          basicAuth:
            username: admin
            password: "{env://KFD_BASIC_AUTH_PASSWORD}"
`

	tempDir, err := os.MkdirTemp("", "furyctl-eks-compat-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "furyctl.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test that existing configuration loads without errors
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	assert.NoError(t, err, "Existing EKSCluster config should load without errors")
}

func testEKSClusterWithFlags(t *testing.T) {
	// Same EKSCluster config with flags added
	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: furyctl-dev-aws
spec:
  distributionVersion: "v1.25.4"
  region: eu-west-1
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: TERRAFORM_TF_STATES_BUCKET_NAME
          keyPrefix: furyctl-next-create-cluster/
          region: eu-west-1
  kubernetes:
    nodeAllowedSshPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAQt/UN/edbCpeWU6M17UqCUqTXs96b7DDWUcbdBrATP"
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
            bucketName: example-velero
            region: eu-west-1

flags:
  global:
    debug: true
    workdir: "/tmp/eks-test"
    gitProtocol: "ssh"
  apply:
    timeout: 7200
    dryRun: false
    vpnAutoConnect: true
    force: ["upgrades"]
  delete:
    dryRun: true
    autoApprove: false
`

	tempDir, err := os.MkdirTemp("", "furyctl-eks-flags-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "furyctl.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test that EKSCluster config with flags loads correctly
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	assert.NoError(t, err, "EKSCluster config with flags should load without errors")
}

func testKFDDistributionExistingConfig(t *testing.T) {
	// Based on fury-getting-started/fury-on-minikube/furyctl.yaml
	config := `apiVersion: kfd.sighup.io/v1alpha2
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
        certManager:
          clusterIssuer:
            name: letsencrypt-fury
            email: example@sighup.io
            type: http01
      logging:
        type: loki
      monitoring:
        type: prometheus
      policy:
        type: none
      dr:
        type: none
        velero: {}
      auth:
        provider:
          type: none
    customPatches:
      patchesStrategicMerge:
        - |
          $patch: delete
          apiVersion: logging-extensions.banzaicloud.io/v1alpha1
          kind: HostTailer
          metadata:
            name: systemd-common
            namespace: logging
        - |
          $patch: delete
          apiVersion: logging-extensions.banzaicloud.io/v1alpha1
          kind: HostTailer
          metadata:
            name: systemd-etcd
            namespace: logging
        - |
          $patch: delete
          apiVersion: apps/v1
          kind: DaemonSet
          metadata:
            name: x509-certificate-exporter-control-plane
            namespace: monitoring
`

	tempDir, err := os.MkdirTemp("", "furyctl-kfd-compat-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "furyctl.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test that existing KFDDistribution configuration loads without errors
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	assert.NoError(t, err, "Existing KFDDistribution config should load without errors")
}

func testKFDDistributionWithFlags(t *testing.T) {
	// Same KFDDistribution config with flags added
	config := `apiVersion: kfd.sighup.io/v1alpha2
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
        certManager:
          clusterIssuer:
            name: letsencrypt-fury
            email: example@sighup.io
            type: http01
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

flags:
  global:
    debug: false
    disableAnalytics: true
    workdir: "{env://PWD}/workspace"
  apply:
    timeout: 3600
    dryRun: false
    skipDepsValidation: false
  tools:
    config: "{file://./furyctl.yaml}"
`

	tempDir, err := os.MkdirTemp("", "furyctl-kfd-flags-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "furyctl.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test that KFDDistribution config with flags loads correctly
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	assert.NoError(t, err, "KFDDistribution config with flags should load without errors")
}

func testOnPremisesExistingConfig(t *testing.T) {
	// Based on typical OnPremises configuration
	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: fury-onpremises
spec:
  distributionVersion: v1.31.0
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: terraform-state-bucket
          keyPrefix: furyctl-onpremises/
          region: eu-west-1
  kubernetes:
    masterNodes:
      hosts:
        - name: master-1
          ip: 10.0.1.10
        - name: master-2
          ip: 10.0.1.11
        - name: master-3
          ip: 10.0.1.12
    workerNodes:
      hosts:
        - name: worker-1
          ip: 10.0.1.20
        - name: worker-2
          ip: 10.0.1.21
    ssh:
      username: fury
      keyPath: ~/.ssh/fury
    loadBalancers:
      enabled: true
      hosts:
        - name: haproxy-1
          ip: 10.0.1.5
        - name: haproxy-2
          ip: 10.0.1.6
      keepalived:
        enabled: true
        interface: eth0
        virtualIp: 10.0.1.4
        virtualIpSubnet: 24
      stats:
        username: admin
        password: "{env://HAPROXY_STATS_PASSWORD}"
  distribution:
    modules:
      ingress:
        baseDomain: internal.fury-demo.local
        nginx:
          type: single
          tls:
            provider: certManager
      logging:
        type: opensearch
      monitoring:
        type: prometheus
      policy:
        type: gatekeeper
      dr:
        type: none
      auth:
        provider:
          type: none
`

	tempDir, err := os.MkdirTemp("", "furyctl-onprem-compat-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "furyctl.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test that existing OnPremises configuration loads without errors
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	assert.NoError(t, err, "Existing OnPremises config should load without errors")
}

func testOnPremisesWithFlags(t *testing.T) {
	// Same OnPremises config with flags added
	config := `apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: fury-onpremises
spec:
  distributionVersion: v1.31.0
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: terraform-state-bucket
          keyPrefix: furyctl-onpremises/
          region: eu-west-1
  kubernetes:
    masterNodes:
      hosts:
        - name: master-1
          ip: 10.0.1.10
    workerNodes:
      hosts:
        - name: worker-1
          ip: 10.0.1.20
    ssh:
      username: fury
      keyPath: ~/.ssh/fury
  distribution:
    modules:
      ingress:
        baseDomain: internal.fury-demo.local
      logging:
        type: opensearch
      monitoring:
        type: prometheus
      dr:
        type: none
      auth:
        provider:
          type: none

flags:
  global:
    debug: true
    workdir: "/tmp/onpremises-test"
    gitProtocol: "https"
  apply:
    timeout: 10800  # 3 hours for OnPremises
    dryRun: false
    skipDepsValidation: false
    force: ["upgrades", "migrations"]
  delete:
    dryRun: true
    autoApprove: false
`

	tempDir, err := os.MkdirTemp("", "furyctl-onprem-flags-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "furyctl.yaml")
	err = os.WriteFile(configPath, []byte(config), 0o644)
	require.NoError(t, err)

	// Test that OnPremises config with flags loads correctly
	manager := NewManager(tempDir)
	err = manager.LoadAndMergeFlags(configPath, "apply")
	assert.NoError(t, err, "OnPremises config with flags should load without errors")
}

// TestBackwardCompatibilityGuarantee tests that the flags feature doesn't break existing functionality
func TestBackwardCompatibilityGuarantee(t *testing.T) {
	testCases := []struct {
		name   string
		config string
	}{
		{
			name: "minimal_eks_config",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: minimal-cluster
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
        baseDomain: example.com
      logging:
        type: opensearch
      monitoring:
        type: prometheus
      dr:
        velero:
          eks:
            bucketName: test-velero
            region: eu-west-1`,
		},
		{
			name: "minimal_kfd_config",
			config: `apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: minimal-kfd
spec:
  distributionVersion: v1.29.0
  distribution:
    kubeconfig: "~/.kube/config"
    modules:
      ingress:
        baseDomain: example.dev
      logging:
        type: loki
      monitoring:
        type: prometheus
      dr:
        type: none
      auth:
        provider:
          type: none`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", fmt.Sprintf("furyctl-backward-compat-%s-*", tc.name))
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			configPath := filepath.Join(tempDir, "furyctl.yaml")
			err = os.WriteFile(configPath, []byte(tc.config), 0o644)
			require.NoError(t, err)

			// Test with different commands
			commands := []string{"global", "apply", "delete", "create"}
			manager := NewManager(tempDir)

			for _, command := range commands {
				t.Run(fmt.Sprintf("command_%s", command), func(t *testing.T) {
					err := manager.LoadAndMergeFlags(configPath, command)
					assert.NoError(t, err,
						"Backward compatibility: existing config should work with command %s", command)
				})
			}
		})
	}
}
