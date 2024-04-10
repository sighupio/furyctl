// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build expensive

package onpremises_test

import (
	"bytes"
	"errors"
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

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/terraform"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	. "github.com/sighupio/furyctl/test/utils"
)

type onPremContextState struct {
	*ContextState
	OnPremCommonDir  string `json:"on_prem_common_dir"`
	TerraformBinPath string `json:"terraform_bin_path"`
	FuryagentBinPath string `json:"furyagent_bin_path"`
	KubectlBinPath   string `json:"kubectl_bin_path"`
}

func newOnPremContextState(tfBinPath, furyAgentBinpath, kubectlBinPath string, state *ContextState) *onPremContextState {
	onPremCommonDir := Must1(filepath.Abs(path.Join(".", "testdata", "common")))

	return &onPremContextState{
		ContextState:     state,
		OnPremCommonDir:  onPremCommonDir,
		TerraformBinPath: tfBinPath,
		FuryagentBinPath: furyAgentBinpath,
		KubectlBinPath:   kubectlBinPath,
	}
}

func TestExpensive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Furyctl Expensive Suite")
}

var (
	furyctl = filepath.Join(Must1(os.MkdirTemp("", "furyctl-expensive-onpremises")), "furyctl")

	assertTimeout = 30 * time.Minute

	assertPollingInterval = 10 * time.Second

	_ = BeforeSuite(CompileFuryctl(furyctl))

	CopyFromTemplate = func(src, dst, clusterName, furyctlCfgPath string) error {
		var cfg template.Config
		var op cluster.OperationPhase

		tmpFolder, err := os.MkdirTemp("", "furyctl-e2e-test-onpremises-infra-")
		if err != nil {
			return fmt.Errorf("error creating temp folder: %w", err)
		}

		defer os.RemoveAll(tmpFolder)

		srcFs := os.DirFS(src)

		if err = iox.CopyRecursive(srcFs, tmpFolder); err != nil {
			return fmt.Errorf("error copying template files: %w", err)
		}

		targetTfDir := path.Join(dst, "infra")
		prefix := "infra"

		cfg.Data = map[string]map[any]any{
			"spec": {
				"clusterName": clusterName,
			},
		}

		err = op.CopyFromTemplate(
			cfg,
			prefix,
			tmpFolder,
			targetTfDir,
			furyctlCfgPath,
		)
		if err != nil {
			return fmt.Errorf("error generating from template files: %w", err)
		}

		return nil
	}

	InitPkis = func(faBinPath, workDir string) error {
		secretsDir := path.Join(workDir, "secrets")

		if _, err := os.Stat(secretsDir); errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(secretsDir, 0o755); err != nil {
				return fmt.Errorf("error creating secrets folder: %w", err)
			}
		}

		faRunner := furyagent.NewRunner(execx.NewStdExecutor(), furyagent.Paths{
			Furyagent: faBinPath,
			WorkDir:   secretsDir,
		})

		if _, err := faRunner.Init("etcd", "--config", "../furyagent-pkis.yml"); err != nil {
			return fmt.Errorf("error running furyagent init: %w", err)
		}

		if _, err := faRunner.Init("master", "--config", "../furyagent-pkis.yml"); err != nil {
			return fmt.Errorf("error running furyagent init: %w", err)
		}

		return nil
	}

	GenOpenVpnClientConfig = func(faBinPath, workDir, outDir string) (string, error) {
		faRunner := furyagent.NewRunner(execx.NewStdExecutor(), furyagent.Paths{
			Furyagent: faBinPath,
			WorkDir:   workDir,
		})

		certName := "test-client"
		certPath := filepath.Join(
			outDir,
			fmt.Sprintf("%s.ovpn", certName),
		)

		out, err := faRunner.ConfigOpenvpnClient(certName)
		if err != nil {
			return "", fmt.Errorf("error running furyagent configure openvpn-client: %w", err)
		}

		err = os.WriteFile(
			certPath,
			out.Bytes(),
			iox.FullRWPermAccess,
		)
		if err != nil {
			return "", fmt.Errorf("error writing openvpn file to disk: %w", err)
		}

		return certPath, nil
	}

	CreateInfra = func(tfBinPath, workDir string) (terraform.OutputJSON, error) {
		timestamp := time.Now().Unix()

		logsPath := path.Join(workDir, "logs")

		outputsPath := path.Join(workDir, "outputs")

		planPath := path.Join(workDir, "plan")

		tfRunner := terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      logsPath,
				Outputs:   outputsPath,
				WorkDir:   workDir,
				Plan:      planPath,
				Terraform: tfBinPath,
			},
		)

		if err := os.Mkdir(logsPath, 0o755); err != nil {
			return nil, fmt.Errorf("error creating terraform logs folder: %w", err)
		}

		if err := os.Mkdir(outputsPath, 0o755); err != nil {
			return nil, fmt.Errorf("error creating terraform outputs folder: %w", err)
		}

		if err := os.Mkdir(planPath, 0o755); err != nil {
			return nil, fmt.Errorf("error creating terraform plan folder: %w", err)
		}

		if err := tfRunner.Init(); err != nil {
			return nil, fmt.Errorf("error running terraform init: %w", err)
		}

		_, err := tfRunner.Plan(timestamp)
		if err != nil {
			return nil, fmt.Errorf("error running terraform plan: %w", err)
		}

		if err := tfRunner.Apply(timestamp); err != nil {
			return nil, fmt.Errorf("cannot create cloud resources: %w", err)
		}

		out, err := tfRunner.Output()
		if err != nil {
			return nil, fmt.Errorf("cannot access terraform apply output: %w", err)
		}

		return out, nil
	}

	DestroyInfra = func(tfBinPath, workDir string) error {
		timestamp := time.Now().Unix()

		logsPath := path.Join(workDir, "logs")

		outputsPath := path.Join(workDir, "outputs")

		planPath := path.Join(workDir, "plan")

		tfRunner := terraform.NewRunner(
			execx.NewStdExecutor(),
			terraform.Paths{
				Logs:      logsPath,
				Outputs:   outputsPath,
				WorkDir:   workDir,
				Plan:      planPath,
				Terraform: tfBinPath,
			},
		)

		if err := tfRunner.Init(); err != nil {
			return fmt.Errorf("error running terraform init: %w", err)
		}

		_, err := tfRunner.Plan(timestamp, "-destroy")
		if err != nil {
			return fmt.Errorf("error running terraform plan: %w", err)
		}

		if err := tfRunner.Destroy(); err != nil {
			return fmt.Errorf("cannot delete cloud resources: %w", err)
		}

		return nil
	}

	ExtractNodeIps = func(tfOut terraform.OutputJSON) ([]string, []string, error) {
		workerPrivateIps := []string{}
		masterPrivateIps := []string{}

		if tfOut["master_private_ips"] == nil {
			return nil, nil, fmt.Errorf("error extracting master private ips")
		}

		mIps, ok := tfOut["master_private_ips"].Value.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("error extracting master private ips")
		}

		for _, ip := range mIps {
			masterPrivateIps = append(masterPrivateIps, ip.(string))
		}

		if tfOut["worker_private_ips"] == nil {
			return nil, nil, fmt.Errorf("error extracting worker private ips")
		}

		wIps, ok := tfOut["worker_private_ips"].Value.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("error extracting worker private ips")
		}

		for _, ip := range wIps {
			workerPrivateIps = append(workerPrivateIps, ip.(string))
		}

		return masterPrivateIps, workerPrivateIps, nil
	}

	InjectNodesData = func(controlPlaneIP, workerOneIP, workerTwoIP, workerThreeIP string) FuryctlYamlCreatorStrategy {
		return func(prevData []byte) []byte {
			data := bytes.ReplaceAll(prevData, []byte("__CONTROL_PLANE_IP__"), []byte(controlPlaneIP))

			data = bytes.ReplaceAll(data, []byte("__NODE_1_IP__"), []byte(workerOneIP))

			data = bytes.ReplaceAll(data, []byte("__NODE_2_IP__"), []byte(workerTwoIP))

			data = bytes.ReplaceAll(data, []byte("__NODE_3_IP__"), []byte(workerThreeIP))

			return data
		}
	}

	FuryctlDeleteCluster = func(cfgPath, distroPath, phase string, dryRun bool, workDir string) *exec.Cmd {
		args := []string{
			"delete",
			"cluster",
			"--config",
			cfgPath,
			"--distro-location",
			distroPath,
			"--debug",
			"--force",
			"all",
			"--workdir",
			workDir,
		}

		if phase != cluster.OperationPhaseAll {
			args = append(args, "--phase", phase)
		}

		if dryRun {
			args = append(args, "--dry-run")
		}

		return exec.Command(furyctl, args...)
	}

	FuryctlCreateCluster = func(configPath, distroPath, phase, skipPhase string, dryRun bool, workDir string) *exec.Cmd {
		args := []string{
			"create",
			"cluster",
			"--config",
			configPath,
			"--distro-location",
			distroPath,
			"--disable-analytics",
			"--debug",
			"--force",
			"all",
			"--skip-vpn-confirmation",
			"--workdir",
			workDir,
		}

		if phase != cluster.OperationPhaseAll {
			args = append(args, "--phase", phase)
		}

		if phase == cluster.OperationPhaseInfrastructure {
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

	ChmodSSHKey = func(workDir string) error {
		if err := os.Chmod(path.Join(workDir, "ssh-private-key.pem"), iox.FullRWPermAccess); err != nil {
			return fmt.Errorf("error changing ssh key permissions: %w", err)
		}

		return nil
	}

	BeforeCreateDeleteTestFunc = func(state *onPremContextState, version string) func() {
		return func() {
			testName := fmt.Sprintf("onpremises-v%s-create-and-delete", version)

			ctxState := NewContextState(testName)

			CreateFuryctlYaml(
				&ctxState,
				"furyctl-minimal.yaml.tpl",
				InjectNodesData("", "", "", ""),
			)

			dlRes := DownloadFuryDistribution(ctxState.FuryctlYaml)

			terraformBinPath := DownloadTerraform(dlRes.DistroManifest.Tools.Common.Terraform.Version)

			furyagentBinPath := DownloadFuryagent(dlRes.DistroManifest.Tools.Common.Furyagent.Version)

			kubectlBinPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)

			*state = *newOnPremContextState(terraformBinPath, furyagentBinPath, kubectlBinPath, &ctxState)

			err := CopyFromTemplate(
				path.Join(state.OnPremCommonDir, "infra"),
				state.TestDir,
				state.ClusterName,
				state.FuryctlYaml,
			)
			Expect(err).To(Not(HaveOccurred()))

			GinkgoWriter.Write([]byte(fmt.Sprintf("Template location: %s", state.TestDir)))

			infraDir := path.Join(state.TestDir, "infra")

			err = InitPkis(furyagentBinPath, infraDir)
			Expect(err).To(Not(HaveOccurred()))

			tfOut, err := CreateInfra(terraformBinPath, infraDir)
			Expect(err).To(Not(HaveOccurred()))

			mIPs, wIPs, err := ExtractNodeIps(tfOut)
			Expect(err).To(Not(HaveOccurred()))
			Expect(len(mIPs)).To(Equal(1))
			Expect(len(wIPs)).To(Equal(3))

			CreateFuryctlYaml(
				state.ContextState,
				"furyctl-minimal.yaml.tpl",
				InjectNodesData(mIPs[0], wIPs[0], wIPs[1], wIPs[2]),
			)

			secretsDir := path.Join(infraDir, "secrets")

			certPath, err := GenOpenVpnClientConfig(furyagentBinPath, secretsDir, state.TestDir)
			Expect(err).To(Not(HaveOccurred()))

			err = ChmodSSHKey(secretsDir)
			Expect(err).To(Not(HaveOccurred()))

			openVPNSession := Must1(ConnectOpenVPN(certPath))
			Eventually(openVPNSession, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	AfterCreateDeleteTestFunc = func(state *onPremContextState) func() {
		return func() {
			infraDir := path.Join(state.TestDir, "infra")

			Must0(DestroyInfra(state.TerraformBinPath, infraDir))

			pkillSession := Must1(KillOpenVPN())

			Eventually(pkillSession, 5*time.Minute, 1*time.Second).Should(gexec.Exit(0))

			err := os.RemoveAll(state.TestDir)
			Expect(err).To(Not(HaveOccurred()))

			err = os.RemoveAll(infraDir)
			Expect(err).To(Not(HaveOccurred()))
		}
	}

	CreateClusterTestFunc = func(state *onPremContextState) func() {
		return func() {
			dlRes := DownloadFuryDistribution(state.FuryctlYaml)

			kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)

			GinkgoWriter.Write([]byte(fmt.Sprintf("Furyctl config path: %s", state.FuryctlYaml)))

			furyctlCreator := NewFuryctlCreator(
				furyctl,
				state.FuryctlYaml,
				state.TestDir,
				false,
			)

			createClusterCmd := furyctlCreator.Create(
				cluster.OperationPhaseAll,
				"",
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

	CreateClusterPhaseKubernetesTestFunc = func(state *onPremContextState) func() {
		return func() {
			GinkgoWriter.Write([]byte(fmt.Sprintf("Furyctl config path: %s", state.FuryctlYaml)))

			furyctlCreator := NewFuryctlCreator(
				furyctl,
				state.FuryctlYaml,
				state.TestDir,
				false,
			)

			createClusterCmd := furyctlCreator.Create(
				cluster.OperationPhaseKubernetes,
				"",
			)

			session := Must1(gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter))

			Consistently(session, 1*time.Minute).ShouldNot(gexec.Exit())

			Eventually(state.Kubeconfig, assertTimeout, assertPollingInterval).Should(BeAnExistingFile())
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	CreateClusterPhaseDistributionTestFunc = func(state *onPremContextState) func() {
		return func() {
			dlRes := DownloadFuryDistribution(state.FuryctlYaml)

			kubectlPath := DownloadKubectl(dlRes.DistroManifest.Tools.Common.Kubectl.Version)

			GinkgoWriter.Write([]byte(fmt.Sprintf("Furyctl config path: %s", state.FuryctlYaml)))

			furyctlCreator := NewFuryctlCreator(
				furyctl,
				state.FuryctlYaml,
				state.TestDir,
				false,
			)

			createClusterCmd := furyctlCreator.Create(
				cluster.OperationPhaseDistribution,
				"",
			)

			session := Must1(gexec.Start(createClusterCmd, GinkgoWriter, GinkgoWriter))

			Consistently(session, 1*time.Minute).ShouldNot(gexec.Exit())

			kubeCmd := exec.Command(kubectlPath, "--kubeconfig", state.Kubeconfig, "get", "nodes")

			kubeSession, err := gexec.Start(kubeCmd, GinkgoWriter, GinkgoWriter)

			Expect(err).To(Not(HaveOccurred()))
			Eventually(kubeSession, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	DeleteClusterTestFunc = func(state *onPremContextState, phase string, ephemeral bool) func() {
		return func() {
			if ephemeral {
				_ = os.RemoveAll(path.Join(state.TestDir, ".furyctl"))
			}

			furyctlDeleter := NewFuryctlDeleter(
				furyctl,
				state.FuryctlYaml,
				state.TestDir,
				false,
			)

			deleteClusterCmd := furyctlDeleter.Delete(
				phase,
			)

			session := Must1(gexec.Start(deleteClusterCmd, GinkgoWriter, GinkgoWriter))
			Eventually(session, assertTimeout, assertPollingInterval).Should(gexec.Exit(0))
		}
	}

	CreateAndDeleteTestScenario = func(version string, ephemeral bool) func() {
		var state *onPremContextState = new(onPremContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal cluster", version)

			Context(contextTitle, Ordered, Serial, Label("slow"), func() {
				BeforeAll(BeforeCreateDeleteTestFunc(state, version))

				AfterAll(AfterCreateDeleteTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster", version), Serial, CreateClusterTestFunc(state))

				It(fmt.Sprintf("should delete a minimal %s cluster", version), Serial, DeleteClusterTestFunc(state, cluster.OperationPhaseAll, ephemeral))
			})
		}
	}

	CreateAndDeleteByPhaseTestScenario = func(version string, ephemeral bool) func() {
		var state *onPremContextState = new(onPremContextState)

		return func() {
			_ = AfterEach(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Write([]byte(fmt.Sprintf("Test for version %s failed, cleaning up...", version)))
				}
			})

			contextTitle := fmt.Sprintf("v%s create and delete a minimal cluster", version)

			Context(contextTitle, Ordered, Serial, Label("slow"), func() {
				BeforeAll(BeforeCreateDeleteTestFunc(state, version))

				AfterAll(AfterCreateDeleteTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster - phase kubernetes", version), Serial, CreateClusterPhaseKubernetesTestFunc(state))

				It(fmt.Sprintf("should create a minimal %s cluster - phase distribution", version), Serial, CreateClusterPhaseDistributionTestFunc(state))

				It(fmt.Sprintf("should delete a minimal %s cluster - phase distribution", version), Serial, DeleteClusterTestFunc(state, cluster.OperationPhaseDistribution, ephemeral))

				It(fmt.Sprintf("should delete a minimal %s cluster - phase kubernetes", version), Serial, DeleteClusterTestFunc(state, cluster.OperationPhaseKubernetes, ephemeral))
			})
		}
	}

	_ = Describe("furyctl & distro v1.25.8 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.8", false))

	_ = Describe("furyctl & distro v1.25.9 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.9", false))

	_ = Describe("furyctl & distro v1.25.10 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.25.10", false))

	_ = Describe("furyctl & distro v1.26.2 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.2", false))

	_ = Describe("furyctl & distro v1.26.3 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.3", false))

	_ = Describe("furyctl & distro v1.26.4 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.4", false))

	_ = Describe("furyctl & distro v1.26.5 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.5", false))

	_ = Describe("furyctl & distro v1.26.6 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.26.6", false))

	_ = Describe("furyctl & distro v1.27.0 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.0", false))

	_ = Describe("furyctl & distro v1.27.1 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.1", false))

	_ = Describe("furyctl & distro v1.27.2 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.2", false))

	_ = Describe("furyctl & distro v1.27.3 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.3", false))

	_ = Describe("furyctl & distro v1.27.4 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.4", false))

	_ = Describe("furyctl & distro v1.27.5 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.27.5", false))

	_ = Describe("furyctl & distro v1.28.0 - minimal", Ordered, Serial, CreateAndDeleteTestScenario("1.28.0", false))

	_ = Describe("furyctl & distro v1.28.0 - minimal - ephemeral", Ordered, Serial, CreateAndDeleteTestScenario("1.28.0", true))

	_ = Describe("furyctl & distro v1.25.9 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.9", false))

	_ = Describe("furyctl & distro v1.25.10 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.25.10", false))

	_ = Describe("furyctl & distro v1.26.2 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.2", false))

	_ = Describe("furyctl & distro v1.26.3 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.3", false))

	_ = Describe("furyctl & distro v1.26.4 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.4", false))

	_ = Describe("furyctl & distro v1.26.5 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.5", false))

	_ = Describe("furyctl & distro v1.26.6 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.26.6", false))

	_ = Describe("furyctl & distro v1.27.0 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.0", false))

	_ = Describe("furyctl & distro v1.27.1 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.1", false))

	_ = Describe("furyctl & distro v1.27.2 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.2", false))

	_ = Describe("furyctl & distro v1.27.3 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.3", false))

	_ = Describe("furyctl & distro v1.27.4 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.4", false))

	_ = Describe("furyctl & distro v1.27.5 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.27.5", false))

	_ = Describe("furyctl & distro v1.28.0 - minimal - by phase", Ordered, Serial, CreateAndDeleteByPhaseTestScenario("1.28.0", false))
)
