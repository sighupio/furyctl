// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck // dot import is required for ginkgo
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck // dot import is required for gomega
	"github.com/onsi/gomega/gexec"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/dependencies/tools"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/tool"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	netx "github.com/sighupio/furyctl/internal/x/net"
	osx "github.com/sighupio/furyctl/internal/x/os"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Conf struct {
	APIVersion string   `validate:"required,api-version"  yaml:"apiVersion"`
	Kind       string   `validate:"required,cluster-kind" yaml:"kind"`
	Metadata   ConfMeta `validate:"required"              yaml:"metadata"`
	Spec       ConfSpec `validate:"required"              yaml:"spec"`
}

type ConfSpec struct {
	DistributionVersion string `validate:"required" yaml:"distributionVersion"`
}

type ConfMeta struct {
	Name string `validate:"required" yaml:"name"`
}

type FuryctlCreator struct {
	furyctl    string
	configPath string
	workDir    string
	outDir     string
	dryRun     bool
}

type FuryctlDeleter struct {
	furyctl    string
	configPath string
	distroPath string
	workDir    string
	outDir     string
	dryRun     bool
}

type ContextState struct {
	TestID      int    `json:"testId"`
	TestName    string `json:"testName"`
	ClusterName string `json:"clusterName"`
	Kubeconfig  string `json:"kubeconfig"`
	FuryctlYaml string `json:"furyctlYaml"`
	DataDir     string `json:"dataDir"`
	DistroDir   string `json:"distroDir"`
	TestDir     string `json:"testDir"`
	TmpDir      string `json:"tmpDir"`
}

const (
	TestIDCeiling = 100000
	BuildWaitTime = 5 * time.Minute
)

var errToolDoesNotSupportDownload = errors.New("does not support download")

func NewContextState(testName string) ContextState {
	testID, err := rand.Int(rand.Reader, big.NewInt(TestIDCeiling))
	if err != nil {
		panic(err)
	}

	clusterName := fmt.Sprintf("furytest-%d", testID.Int64())

	homeDir, dataDir, tmpDir := PrepareDirs(testName)

	testDir := path.Join(homeDir, ".furyctl", "tests", testName)
	testState := path.Join(testDir, fmt.Sprintf("%s.teststate", clusterName))

	Must0(os.MkdirAll(testDir, iox.FullPermAccess))

	furyctlYaml := path.Join(testDir, fmt.Sprintf("%s.yaml", clusterName))

	s := ContextState{
		TestID:      int(testID.Int64()),
		TestName:    testName,
		ClusterName: clusterName,
		FuryctlYaml: furyctlYaml,
		DataDir:     dataDir,
		TestDir:     testDir,
		TmpDir:      tmpDir,
	}

	Must0(os.WriteFile(testState, Must1(json.Marshal(s)), iox.RWPermAccess))

	return s
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

	dataDir := Must1(filepath.Abs(path.Join(".", "testdata", strings.ReplaceAll(name, ".", "-"))))

	tmpDir := Must1(os.MkdirTemp("", name))

	return homeDir, dataDir, tmpDir
}

func Copy(src, dst string) {
	input := Must1(os.ReadFile(src))

	Must0(os.WriteFile(dst, input, iox.RWPermAccess))
}

func CompileFuryctl(outputPath string) func() {
	return func() {
		cmd := exec.Command("go", "build", "-o", outputPath, "../../../main.go")

		session := Must1(gexec.Start(cmd, GinkgoWriter, GinkgoWriter))

		Eventually(session, BuildWaitTime).Should(gexec.Exit(0))
	}
}

func DownloadFuryDistribution(furyctlConfPath string) distribution.DownloadResult {
	distrodl := distribution.NewDownloader(netx.NewGoGetterClient(), true)

	return Must1(distrodl.Download("", furyctlConfPath))
}

func Download(toolName, version string) string {
	binPath := filepath.Join(os.TempDir(), "bin")

	toolFactory := tools.NewFactory(execx.NewStdExecutor(), tools.FactoryPaths{Bin: binPath})

	client := netx.NewGoGetterClient()

	tfc := toolFactory.Create(tool.Name(toolName), version)
	if tfc == nil || !tfc.SupportsDownload() {
		panic(fmt.Errorf("tool '%s' %w", toolName, errToolDoesNotSupportDownload))
	}

	dst := filepath.Join(binPath, toolName, version)

	if err := client.Download(tfc.SrcPath(), dst); err != nil {
		panic(fmt.Errorf("%w '%s': %v", distribution.ErrDownloadingFolder, tfc.SrcPath(), err))
	}

	if err := tfc.Rename(dst); err != nil {
		panic(fmt.Errorf("%w '%s': %v", distribution.ErrRenamingFile, tfc.SrcPath(), err))
	}

	if err := os.Chmod(filepath.Join(dst, toolName), iox.FullPermAccess); err != nil {
		panic(fmt.Errorf("%w '%s': %v", distribution.ErrChangingFilePermissions, tfc.SrcPath(), err))
	}

	return path.Join(dst, toolName)
}

func DownloadKubectl(version string) string {
	return Download("kubectl", version)
}

func DownloadTerraform(version string) string {
	return Download("terraform", version)
}

func DownloadFuryagent(version string) string {
	return Download("furyagent", version)
}

type FuryctlYamlCreatorStrategy func(prevData []byte) []byte

func FuryctlYamlCreatorIdentityStrategy(prevData []byte) []byte {
	return prevData
}

func CreateFuryctlYaml(s *ContextState, furyctlYamlTplName string, strategy FuryctlYamlCreatorStrategy) {
	if strategy == nil {
		strategy = FuryctlYamlCreatorIdentityStrategy
	}

	furyctlYamlTplPath := path.Join(s.DataDir, furyctlYamlTplName)

	tplData := Must1(os.ReadFile(furyctlYamlTplPath))

	data := bytes.ReplaceAll(tplData, []byte("__CLUSTER_NAME__"), []byte(s.ClusterName))

	data = strategy(data)

	Must0(os.WriteFile(s.FuryctlYaml, data, iox.FullPermAccess))
}

func LoadFuryCtl(furyctlYamlPath string) Conf {
	return Must1(yamlx.FromFileV3[Conf](furyctlYamlPath))
}

func ConnectOpenVPN(certPath string) (*gexec.Session, error) {
	var cmd *exec.Cmd

	isRoot, err := osx.IsRoot()
	if err != nil {
		return nil, fmt.Errorf("error checking if user is root: %w", err)
	}

	if isRoot {
		cmd = exec.Command("openvpn", "--config", certPath, "--daemon")
	} else {
		cmd = exec.Command("sudo", "openvpn", "--config", certPath, "--daemon")
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return nil, fmt.Errorf("error connecting to openvpn: %w", err)
	}

	return session, nil
}

func KillOpenVPN() (*gexec.Session, error) {
	var cmd *exec.Cmd

	isRoot, err := osx.IsRoot()
	if err != nil {
		return nil, fmt.Errorf("error checking if user is root: %w", err)
	}

	if isRoot {
		cmd = exec.Command("pkill", "openvpn")
	} else {
		cmd = exec.Command("sudo", "pkill", "openvpn")
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return nil, fmt.Errorf("error killing openvpn: %w", err)
	}

	return session, nil
}

func NewFuryctlCreator(furyctl, configPath, workDir, outDir string, dryRun bool) *FuryctlCreator {
	return &FuryctlCreator{
		furyctl:    furyctl,
		configPath: configPath,
		workDir:    workDir,
		outDir:     outDir,
		dryRun:     dryRun,
	}
}

func (f *FuryctlCreator) Create(phase, startFrom string) *exec.Cmd {
	args := []string{
		"create",
		"cluster",
		"--config",
		f.configPath,
		"--disable-analytics",
		"--debug",
		"--force",
		"--skip-vpn-confirmation",
		"--workdir",
		f.workDir,
		"--outdir",
		f.outDir,
	}

	if phase != cluster.OperationPhaseAll {
		args = append(args, "--phase", phase)
	}

	if phase == cluster.OperationPhaseInfrastructure {
		args = append(args, "--vpn-auto-connect")
	}

	if startFrom != "" {
		args = append(args, "--start-from", startFrom)
	}

	if f.dryRun {
		args = append(args, "--dry-run")
	}

	return exec.Command(f.furyctl, args...)
}

func NewFuryctlDeleter(
	furyctl,
	configPath,
	distroPath,
	workDir,
	outDir string,
	dryRun bool,
) *FuryctlDeleter {
	return &FuryctlDeleter{
		furyctl:    furyctl,
		configPath: configPath,
		distroPath: distroPath,
		workDir:    workDir,
		outDir:     outDir,
		dryRun:     dryRun,
	}
}

func (f *FuryctlDeleter) Delete(phase string) *exec.Cmd {
	args := []string{
		"delete",
		"cluster",
		"--config",
		f.configPath,
		"--distro-location",
		f.distroPath,
		"--debug",
		"--force",
		"--workdir",
		f.workDir,
		"--outdir",
		f.outDir,
	}

	if phase != cluster.OperationPhaseAll {
		args = append(args, "--phase", phase)
	}

	if f.dryRun {
		args = append(args, "--dry-run")
	}

	return exec.Command(f.furyctl, args...)
}
