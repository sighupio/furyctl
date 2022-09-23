package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	tJson "github.com/hashicorp/terraform-json"
	schm "github.com/sighupio/fury-distribution/pkg/schemas"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/yaml"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ErrUnsupportedPhase = fmt.Errorf("unsupported phase")

type V1alpha2 struct {
	Phase          string
	KfdManifest    distribution.Manifest
	ConfigPath     string
	VpnAutoConnect bool
}

func (v *V1alpha2) WithPhase(phase string) ClusterCreator {
	v.Phase = phase
	return v
}

func (v *V1alpha2) WithKfdManifest(kfdManifest distribution.Manifest) ClusterCreator {
	v.KfdManifest = kfdManifest
	return v
}

func (v *V1alpha2) WithConfigPath(configPath string) ClusterCreator {
	v.ConfigPath = configPath
	return v
}

func (v *V1alpha2) WithVpnAutoConnect(vpnAutoConnect bool) ClusterCreator {
	v.VpnAutoConnect = vpnAutoConnect
	return v
}

func (v *V1alpha2) Create() error {
	logrus.Infof("Running phase: %s", v.Phase)

	switch v.Phase {
	case "infrastructure":
		return v.Infrastructure()
	case "kubernetes":
		return v.Kubernetes()
	case "distribution":
		return v.Distribution()
	case "":
		err := v.Infrastructure()
		if err != nil {
			return err
		}

		err = v.Kubernetes()
		if err != nil {
			return err
		}

		return v.Distribution()
	default:
		return ErrUnsupportedPhase
	}
}

func (v *V1alpha2) Infrastructure() error {
	var config template.Config

	infraPath := path.Join(".infrastructure")

	err := os.Mkdir(infraPath, 0o755)
	if err != nil {
		return err
	}

	sourceTfDir := path.Join("configs", "provisioners", "bootstrap", "aws")
	targetTfDir := path.Join(infraPath, "terraform")

	outDirPath, err := os.MkdirTemp("", "furyctl-infra-")
	if err != nil {
		return err
	}

	tfConfVars := map[string]map[any]any{
		"kubernetes": {
			"eks": v.KfdManifest.Kubernetes.Eks,
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

	logrus.Infoln("Generating directory structure")

	planPath := path.Join(infraPath, "terraform", "plan")
	logsPath := path.Join(infraPath, "terraform", "logs")
	outputsPath := path.Join(infraPath, "terraform", "outputs")
	secretsPath := path.Join(infraPath, "terraform", "secrets")

	binPath, err := filepath.Abs("./vendor")
	if err != nil {
		return err
	}

	err = os.Mkdir(planPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(logsPath, 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir(outputsPath, 0o755)
	if err != nil {
		return err
	}

	logrus.Infoln("Generating tvars file")

	var buffer bytes.Buffer

	furyFile, err := yaml.FromFileV3[schm.EksclusterKfdV1Alpha2Json](v.ConfigPath)
	if err != nil {
		return err
	}

	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", furyFile.Metadata.Name))
	buffer.WriteString(fmt.Sprintf(
		"network_cidr = \"%v\"\n",
		furyFile.Spec.Infrastructure.Vpc.Network.Cidr,
	))

	publicSubnetworkCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public))

	for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Public {
		publicSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	privateSubnetworkCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private))

	for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Network.SubnetsCidrs.Private {
		privateSubnetworkCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
	}

	buffer.WriteString(fmt.Sprintf(
		"public_subnetwork_cidrs = [%v]\n",
		strings.Join(publicSubnetworkCidrs, ",")))

	buffer.WriteString(fmt.Sprintf(
		"private_subnetwork_cidrs = [%v]\n",
		strings.Join(privateSubnetworkCidrs, ",")))

	if furyFile.Spec.Infrastructure.Vpc.Vpn != nil {
		buffer.WriteString(fmt.Sprintf("vpn_subnetwork_cidr = \"%v\"\n", furyFile.Spec.Infrastructure.Vpc.Vpn.VpnClientsSubnetCidr))
		buffer.WriteString(fmt.Sprintf("vpn_instances = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.Instances))

		if furyFile.Spec.Infrastructure.Vpc.Vpn.Port != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_port = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.Port))
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.InstanceType != "" {
			buffer.WriteString(fmt.Sprintf("vpn_instance_type = \"%v\"\n", furyFile.Spec.Infrastructure.Vpc.Vpn.InstanceType))
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.DiskSize != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_instance_disk_size = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.DiskSize))
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.OperatorName != "" {
			buffer.WriteString(fmt.Sprintf("vpn_operator_name = \"%v\"\n", furyFile.Spec.Infrastructure.Vpc.Vpn.OperatorName))
		}

		if furyFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits != 0 {
			buffer.WriteString(fmt.Sprintf("vpn_dhparams_bits = %v\n", furyFile.Spec.Infrastructure.Vpc.Vpn.DhParamsBits))
		}

		if len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs) != 0 {
			allowedCidrs := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs))

			for i, cidr := range furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.AllowedFromCidrs {
				allowedCidrs[i] = fmt.Sprintf("\"%v\"", cidr)
			}

			buffer.WriteString(fmt.Sprintf("vpn_operator_cidrs = [%v]\n", strings.Join(allowedCidrs, ",")))
		}

		if len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName) != 0 {
			githubUsers := make([]string, len(furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName))

			for i, gu := range furyFile.Spec.Infrastructure.Vpc.Vpn.Ssh.GithubUsersName {
				githubUsers[i] = fmt.Sprintf("\"%v\"", gu)
			}

			buffer.WriteString(fmt.Sprintf("vpn_ssh_users = [%v]\n", strings.Join(githubUsers, ",")))
		}
	}

	targetTfVars := path.Join(infraPath, "terraform", "main.auto.tfvars")

	err = os.WriteFile(targetTfVars, buffer.Bytes(), 0o600)
	if err != nil {
		return err
	}

	terraformBinPath := path.Join(binPath, "bin", "terraform")
	furyAgentBinPath := path.Join(binPath, "bin", "furyagent")

	terraformInitCmd := exec.Command(terraformBinPath, "init")
	terraformInitCmd.Stdout = os.Stdout
	terraformInitCmd.Stderr = os.Stderr
	terraformInitCmd.Dir = path.Join(infraPath, "terraform")

	err = terraformInitCmd.Run()
	if err != nil {
		return err
	}

	var planBuffer bytes.Buffer

	terraformPlanCmd := exec.Command(terraformBinPath, "plan", "--out=plan/terraform.plan", "-no-color")
	terraformPlanCmd.Stdout = io.MultiWriter(os.Stdout, &planBuffer)
	terraformPlanCmd.Stderr = os.Stderr
	terraformPlanCmd.Dir = path.Join(infraPath, "terraform")

	err = terraformPlanCmd.Run()
	if err != nil {
		return err
	}

	timestamp := time.Now().Unix()

	err = os.WriteFile(path.Join(planPath, fmt.Sprintf("plan-%d.log", timestamp)), planBuffer.Bytes(), 0o600)
	if err != nil {
		return err
	}

	var applyBuffer bytes.Buffer

	terraformApplyCmd := exec.Command(terraformBinPath, "apply", "-no-color", "-json", "plan/terraform.plan")
	terraformApplyCmd.Stdout = io.MultiWriter(os.Stdout, &applyBuffer)
	terraformApplyCmd.Stderr = os.Stderr
	terraformApplyCmd.Dir = path.Join(infraPath, "terraform")

	err = terraformApplyCmd.Run()
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(logsPath, fmt.Sprintf("%d.log", timestamp)), applyBuffer.Bytes(), 0o600)
	if err != nil {
		return err
	}

	var applyLogOut struct {
		Outputs map[string]*tJson.StateOutput `json:"outputs"`
	}

	parsedApplyLog, err := ioutil.ReadFile(path.Join(logsPath, fmt.Sprintf("%d.log", timestamp)))
	if err != nil {
		return err
	}

	applyLog := string(parsedApplyLog)

	pattern := regexp.MustCompile("(\"outputs\":){(.*?)}}")

	outputsStringIndex := pattern.FindStringIndex(applyLog)
	if outputsStringIndex == nil {
		return fmt.Errorf("can't get outputs from terraform apply logs")
	}

	outputsString := fmt.Sprintf("{%s}", applyLog[outputsStringIndex[0]:outputsStringIndex[1]])

	// Checking if the outputs are valid json
	err = json.Unmarshal([]byte(outputsString), &applyLogOut)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path.Join(outputsPath, "output.json"), []byte(outputsString), 0o600)
	if err != nil {
		return err
	}

	if furyFile.Spec.Infrastructure.Vpc.Vpn != nil && furyFile.Spec.Infrastructure.Vpc.Vpn.Instances > 0 {
		var furyAgentBuffer bytes.Buffer

		clientName := furyFile.Metadata.Name

		whoamiResp, err := exec.Command("whoami").Output()
		if err != nil {
			return err
		}

		whoami := strings.TrimSpace(string(whoamiResp))
		clientName = fmt.Sprintf("%s-%s", clientName, whoami)

		furyAgentCmd := exec.Command(furyAgentBinPath,
			"configure",
			"openvpn-client",
			fmt.Sprintf("--client-name=%s", clientName),
			"--config=furyagent.yml",
		)
		furyAgentCmd.Stdout = io.MultiWriter(os.Stdout, &furyAgentBuffer)
		furyAgentCmd.Stderr = os.Stderr
		furyAgentCmd.Dir = secretsPath

		err = furyAgentCmd.Run()
		if err != nil {
			return err
		}

		err = os.WriteFile(path.Join(secretsPath, fmt.Sprintf("%s.ovpn", clientName)), furyAgentBuffer.Bytes(), 0o600)
		if err != nil {
			return err
		}

		if v.VpnAutoConnect {
			openVpnCmd := exec.Command(
				"openvpn",
				"--config",
				fmt.Sprintf("%s.ovpn", clientName),
				"--daemon",
			)
			openVpnCmd.Stdout = os.Stdout
			openVpnCmd.Stderr = os.Stderr
			openVpnCmd.Dir = secretsPath

			err = openVpnCmd.Run()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *V1alpha2) Kubernetes() error {
	return nil
}

func (v *V1alpha2) Distribution() error {
	return nil
}
