package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
)

type Kubernetes struct {
	Path          string
	TerraformPath string
	PlanPath      string
	LogsPath      string
	OutputsPath   string
	SecretsPath   string
}

func NewKubernetes() (*Kubernetes, error) {
	infraPath := path.Join(".kubernetes")

	binPath, err := filepath.Abs("./vendor")
	if err != nil {
		return &Kubernetes{}, err
	}

	terraformPath := path.Join(binPath, "bin", "terraform")

	planPath := path.Join(infraPath, "terraform", "plan")
	logsPath := path.Join(infraPath, "terraform", "logs")
	outputsPath := path.Join(infraPath, "terraform", "outputs")
	secretsPath := path.Join(infraPath, "terraform", "secrets")

	return &Kubernetes{
		Path:          infraPath,
		TerraformPath: terraformPath,
		PlanPath:      planPath,
		LogsPath:      logsPath,
		OutputsPath:   outputsPath,
		SecretsPath:   secretsPath,
	}, nil
}

func (k *Kubernetes) CreateFolder() error {
	return os.Mkdir(k.Path, 0o755)
}

func (k *Kubernetes) CopyFromTemplate(kfdManifest config.KFD) error {
	var config template.Config

	sourceTfDir := path.Join("configs", "provisioners", "cluster", "eks")
	targetTfDir := path.Join(k.Path, "terraform")

	outDirPath, err := os.MkdirTemp("", "furyctl-kube-")
	if err != nil {
		return err
	}

	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": kfdManifest.Kubernetes.Eks,
		},
	}

	config.Data = tfConfVars

	tfConfigPath := path.Join(outDirPath, "tf-config.yaml")
	tfConfig, err := yaml.MarshalV2(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(tfConfigPath, tfConfig, 0o644)
	if err != nil {
		return err
	}

	templateModel, err := template.NewTemplateModel(
		sourceTfDir,
		targetTfDir,
		tfConfigPath,
		outDirPath,
		".tpl",
		true,
		false,
	)
	if err != nil {
		return err
	}

	err = templateModel.Generate()
	return nil
}

func (k *Kubernetes) CreateFolderStructure() error {
	err := os.Mkdir(k.PlanPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(k.LogsPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(k.SecretsPath, 0o755)
	if err != nil {
		return err
	}

	return os.Mkdir(k.OutputsPath, 0o755)
}

func (k *Kubernetes) TerraformInit() error {
	terraformInitCmd := exec.Command(k.TerraformPath, "init")
	terraformInitCmd.Stdout = os.Stdout
	terraformInitCmd.Stderr = os.Stderr
	terraformInitCmd.Dir = path.Join(k.Path, "terraform")

	return terraformInitCmd.Run()
}

func (k *Kubernetes) TerraformPlan(timestamp int64) error {
	var planBuffer bytes.Buffer

	terraformPlanCmd := exec.Command(k.TerraformPath, "plan", "--out=plan/terraform.plan", "-no-color")
	terraformPlanCmd.Stdout = io.MultiWriter(os.Stdout, &planBuffer)
	terraformPlanCmd.Stderr = os.Stderr
	terraformPlanCmd.Dir = path.Join(k.Path, "terraform")

	err := terraformPlanCmd.Run()
	if err != nil {
		return err
	}

	logFilePath := fmt.Sprintf("plan-%d.log", timestamp)

	return os.WriteFile(path.Join(k.PlanPath, logFilePath), planBuffer.Bytes(), 0o600)
}

func (k *Kubernetes) TerraformApply(timestamp int64) (OutputJson, error) {
	var applyBuffer bytes.Buffer
	var applyLogOut OutputJson

	terraformApplyCmd := exec.Command(k.TerraformPath, "apply", "-no-color", "-json", "plan/terraform.plan")
	terraformApplyCmd.Stdout = io.MultiWriter(os.Stdout, &applyBuffer)
	terraformApplyCmd.Stderr = os.Stderr
	terraformApplyCmd.Dir = path.Join(k.Path, "terraform")

	err := terraformApplyCmd.Run()
	if err != nil {
		return applyLogOut, err
	}

	err = os.WriteFile(path.Join(k.LogsPath, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return applyLogOut, err
	}

	parsedApplyLog, err := os.ReadFile(path.Join(k.LogsPath, fmt.Sprintf("%d.log", timestamp)))
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

	err = os.WriteFile(path.Join(k.OutputsPath, "output.json"), []byte(outputsString), 0o600)

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

	return os.WriteFile(path.Join(k.SecretsPath, "kubeconfig"), []byte(kubeString), 0o600)
}

func (k *Kubernetes) SetKubeconfigEnv() error {
	kubePath, err := filepath.Abs(path.Join(k.SecretsPath, "kubeconfig"))
	if err != nil {
		return err
	}

	return os.Setenv("KUBECONFIG", kubePath)
}
