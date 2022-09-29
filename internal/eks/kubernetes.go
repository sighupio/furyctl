package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/template"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
)

type Kubernetes struct {
	base *Base
}

func NewKubernetes() (*Kubernetes, error) {
	base, err := NewBase(".kubernetes")
	if err != nil {
		return nil, err
	}

	return &Kubernetes{
		base: base,
	}, nil
}

func (k *Kubernetes) CreateFolder() error {
	return k.base.CreateFolder()
}

func (k *Kubernetes) CreateFolderStructure() error {
	return k.base.CreateFolderStructure()
}

func (k *Kubernetes) Path() string {
	return k.base.Path
}

func (k *Kubernetes) CopyFromTemplate(kfdManifest config.KFD) error {
	var cfg template.Config

	sourceTfDir := path.Join("configs", "provisioners", "cluster", "eks")
	targetTfDir := path.Join(k.base.Path, "terraform")
	prefix := "kube"
	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": kfdManifest.Kubernetes.Eks,
		},
	}

	cfg.Data = tfConfVars

	return k.base.CopyFromTemplate(
		cfg,
		prefix,
		sourceTfDir,
		targetTfDir,
	)
}

func (k *Kubernetes) TerraformInit() error {
	terraformInitCmd := exec.Command(k.base.TerraformPath, "init")
	terraformInitCmd.Stdout = os.Stdout
	terraformInitCmd.Stderr = os.Stderr
	terraformInitCmd.Dir = path.Join(k.base.Path, "terraform")

	return terraformInitCmd.Run()
}

func (k *Kubernetes) TerraformPlan(timestamp int64) error {
	var planBuffer bytes.Buffer

	terraformPlanCmd := exec.Command(k.base.TerraformPath, "plan", "--out=plan/terraform.plan", "-no-color")
	terraformPlanCmd.Stdout = io.MultiWriter(os.Stdout, &planBuffer)
	terraformPlanCmd.Stderr = os.Stderr
	terraformPlanCmd.Dir = path.Join(k.base.Path, "terraform")

	err := terraformPlanCmd.Run()
	if err != nil {
		return err
	}

	logFilePath := fmt.Sprintf("plan-%d.log", timestamp)

	return os.WriteFile(path.Join(k.base.PlanPath, logFilePath), planBuffer.Bytes(), 0o600)
}

func (k *Kubernetes) TerraformApply(timestamp int64) (OutputJson, error) {
	var applyBuffer bytes.Buffer
	var applyLogOut OutputJson

	terraformApplyCmd := exec.Command(k.base.TerraformPath, "apply", "-no-color", "-json", "plan/terraform.plan")
	terraformApplyCmd.Stdout = io.MultiWriter(os.Stdout, &applyBuffer)
	terraformApplyCmd.Stderr = os.Stderr
	terraformApplyCmd.Dir = path.Join(k.base.Path, "terraform")

	err := terraformApplyCmd.Run()
	if err != nil {
		return applyLogOut, err
	}

	err = os.WriteFile(path.Join(k.base.LogsPath, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return applyLogOut, err
	}

	parsedApplyLog, err := os.ReadFile(path.Join(k.base.LogsPath, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return applyLogOut, err
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return applyLogOut, fmt.Errorf("can't get outputs from terraform apply logs")
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	err = json.Unmarshal([]byte(outputsString), &applyLogOut)
	if err != nil {
		return applyLogOut, err
	}

	err = os.WriteFile(path.Join(k.base.OutputsPath, "output.json"), []byte(outputsString), 0o600)

	return applyLogOut, err
}

func (k *Kubernetes) CreateKubeconfig(o OutputJson) error {
	if o.Outputs["kubeconfig"] == nil {
		return fmt.Errorf("can't get kubeconfig from terraform apply logs")
	}

	kubeString, ok := o.Outputs["kubeconfig"].Value.(string)
	if !ok {
		return fmt.Errorf("can't get kubeconfig from terraform apply logs")
	}

	return os.WriteFile(path.Join(k.base.SecretsPath, "kubeconfig"), []byte(kubeString), 0o600)
}

func (k *Kubernetes) SetKubeconfigEnv() error {
	kubePath, err := filepath.Abs(path.Join(k.base.SecretsPath, "kubeconfig"))
	if err != nil {
		return err
	}

	return os.Setenv("KUBECONFIG", kubePath)
}
