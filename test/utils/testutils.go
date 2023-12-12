package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
	osx "github.com/sighupio/furyctl/internal/x/os"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"

	"github.com/sighupio/furyctl/internal/distribution"
)

type Conf struct {
	APIVersion string   `yaml:"apiVersion" validate:"required,api-version"`
	Kind       string   `yaml:"kind"       validate:"required,cluster-kind"`
	Metadata   ConfMeta `yaml:"metadata"   validate:"required"`
	Spec       ConfSpec `yaml:"spec"       validate:"required"`
}

type ConfSpec struct {
	DistributionVersion string `yaml:"distributionVersion" validate:"required"`
}

type ConfMeta struct {
	Name string `yaml:"name" validate:"required"`
}

type ContextState struct {
	TestId      int
	TestName    string
	ClusterName string
	Kubeconfig  string
	FuryctlYaml string
	HomeDir     string
	DataDir     string
	DistroDir   string
	TestDir     string
	TmpDir      string
}

var (
	client = netx.NewGoGetterClient()

	distrodl = distribution.NewDownloader(client, true)

	binPath = filepath.Join(os.TempDir(), "bin")

	toolFactory = tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{Bin: binPath})
)

func NewContextState(testName string) *ContextState {
	testId := rand.Intn(100000)
	clusterName := fmt.Sprintf("furytest-%d", testId)

	homeDir, dataDir, tmpDir := PrepareDirs(testName)

	testDir := path.Join(homeDir, ".furyctl", "tests", testName)
	testState := path.Join(testDir, fmt.Sprintf("%s.teststate", clusterName))

	Must0(os.MkdirAll(testDir, 0o755))

	kubeconfig := path.Join(
		homeDir,
		".furyctl",
		clusterName,
		cluster.OperationPhaseKubernetes,
		"terraform",
		"secrets",
		"kubeconfig",
	)

	furyctlYaml := path.Join(testDir, fmt.Sprintf("%s.yaml", clusterName))

	s := ContextState{
		TestId:      testId,
		TestName:    testName,
		ClusterName: clusterName,
		Kubeconfig:  kubeconfig,
		FuryctlYaml: furyctlYaml,
		HomeDir:     homeDir,
		DataDir:     dataDir,
		TestDir:     testDir,
		TmpDir:      tmpDir,
	}

	Must0(os.WriteFile(testState, Must1(json.Marshal(s)), 0o644))

	return &s
}

func Must0(err error) {
	if err != nil {
		panic(err)
	}
}

func Must1[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

func PrepareDirs(name string) (string, string, string) {
	homeDir := Must1(os.UserHomeDir())

	dataDir := Must1(filepath.Abs(path.Join(".", "testdata", name)))

	tmpDir := Must1(os.MkdirTemp("", name))

	return homeDir, dataDir, tmpDir
}

func Copy(src, dst string) {
	input := Must1(os.ReadFile(src))

	Must0(os.WriteFile(dst, input, 0o644))
}

func CompileFuryctl(outputPath string) func() {
	return func() {
		cmd := exec.Command("go", "build", "-o", outputPath, "../../../main.go")

		session := Must1(gexec.Start(cmd, GinkgoWriter, GinkgoWriter))

		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
	}
}

func DownloadFuryDistribution(furyctlConfPath string) distribution.DownloadResult {
	return Must1(distrodl.Download("", furyctlConfPath))
}

func DownloadKubectl(version string) string {
	name := "kubectl"

	tfc := toolFactory.Create(tool.Name(name), version)
	if tfc == nil || !tfc.SupportsDownload() {
		panic(fmt.Errorf("tool '%s' does not support download", name))
	}

	dst := filepath.Join(binPath, name, version)

	if err := client.Download(tfc.SrcPath(), dst); err != nil {
		panic(fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, tfc.SrcPath(), err))
	}

	if err := tfc.Rename(dst); err != nil {
		panic(fmt.Errorf("%w '%s': %v", distribution.ErrRenamingFile, tfc.SrcPath(), err))
	}

	if err := os.Chmod(filepath.Join(dst, name), iox.FullPermAccess); err != nil {
		panic(fmt.Errorf("%w '%s': %v", distribution.ErrChangingFilePermissions, tfc.SrcPath(), err))
	}

	return path.Join(dst, name)
}

func CreateFuryctlYaml(s *ContextState, furyctlYamlTplName string) {
	furyctlYamlTplPath := path.Join(s.DataDir, furyctlYamlTplName)

	tplData := Must1(os.ReadFile(furyctlYamlTplPath))

	data := bytes.ReplaceAll(tplData, []byte("__CLUSTER_NAME__"), []byte(s.ClusterName))

	Must0(os.WriteFile(s.FuryctlYaml, data, iox.FullPermAccess))
}

func LoadFuryCtl(furyctlYamlPath string) Conf {
	return Must1(yamlx.FromFileV3[Conf](furyctlYamlPath))
}

func KillOpenVPN() (*gexec.Session, error) {
	var cmd *exec.Cmd

	isRoot, err := osx.IsRoot()
	if err != nil {
		return nil, err
	}

	if isRoot {
		cmd = exec.Command("pkill", "openvpn")
	} else {
		cmd = exec.Command("sudo", "pkill", "openvpn")
	}

	return gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
}

func FuryctlCreateCluster(furyctl, configPath, phase, skipPhase string, dryRun bool, workDir string) *exec.Cmd {
	args := []string{
		"create",
		"cluster",
		"--config",
		configPath,
		// "--distro-location",
		// distroPath,
		"--disable-analytics",
		"--debug",
		"--force",
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

func FuryctlDeleteCluster(furyctl, cfgPath, distroPath, phase string, dryRun bool, workDir string) *exec.Cmd {
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
