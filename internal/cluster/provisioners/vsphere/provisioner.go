// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsphere

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gobuffalo/packr/v2"
	getter "github.com/hashicorp/go-getter"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/relex/aini"
	cfg "github.com/sighupio/furyctl/internal/cluster/configuration"
	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sirupsen/logrus"
	"google.golang.org/appengine/log"
)

// VSphere represents the VSphere provisioner
type VSphere struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

// InitMessage return a custom provisioner message the user will see once the cluster is ready to be updated
func (e *VSphere) InitMessage() string {
	return `[VSphere] Fury

This provisioner creates a battle-tested Kubernetes vSphere Cluster
with a private and production-grade setup.

It will deploy all the components required to run a Kubernetes Cluster:
- Load Balancer (Control Plane & Infrastructure components)
- Kubernetes Control Plane
- Dedicated intrastructure nodes
- General node pools

Requires to connect to a VPN server to deploy the cluster from this computer.
Use a bastion host (inside the same vSphere network) as an alternative method to deploy the cluster.

The provisioner requires the following software installed:
- ansible

And internet connection to download remote repositories from the SIGHUP enterprise repositories.
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (e *VSphere) UpdateMessage() string {
	var output map[string]tfexec.OutputMeta
	output, err := e.terraform.Output(context.Background())
	if err != nil {
		logrus.Error("Can not get output values")
	}
	var inventoryOutput string
	err = json.Unmarshal(output["ansible_inventory"].Value, &inventoryOutput)
	if err != nil {
		logrus.Error("Can not get `ansible_inventory` value")
	}
	inventory, _ := aini.Parse(strings.NewReader(inventoryOutput))
	kubernetes_control_plane_address := strings.Replace(
		inventory.Groups["all"].Vars["kubernetes_control_plane_address"],
		"\"",
		"",
		-1,
	)
	clusterOperatorName := strings.Replace(inventory.Groups["all"].Vars["ansible_user"], "\"", "", -1)

	return fmt.Sprintf(
		`[vSphere] Fury

All the cluster components are up to date.
vSphere Kubernetes cluster ready.

vSphere Cluster Endpoint: %v
SSH Operator Name: %v

Use the ssh %v username to access the vSphere instances with the configured SSH key.
Discover the instances by running

$ kubectl get nodes

Then access by running:

$ ssh %v@node-name-reported-by-kubectl-get-nodes

`, kubernetes_control_plane_address, clusterOperatorName, clusterOperatorName, clusterOperatorName,
	)
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (e *VSphere) DestroyMessage() string {
	return `[VSphere] Fury
All cluster components were destroyed.
vSphere control plane, load balancer and workers went away.

Had problems, contact us at sales@sighup.io.
`
}

// Enterprise return a boolean indicating it is an enterprise provisioner
func (e *VSphere) Enterprise() bool {
	return true
}

const (
	projectPath = "../../../../data/provisioners/cluster/vsphere"
)

func (e VSphere) createVarFile() (err error) {
	var buffer bytes.Buffer
	spec := e.config.Spec.(cfg.VSphere)
	buffer.WriteString(fmt.Sprintf("name = \"%v\"\n", e.config.Metadata.Name))
	buffer.WriteString(fmt.Sprintf("kube_version = \"%v\"\n", spec.Version))
	buffer.WriteString(fmt.Sprintf("kube_control_plane_endpoint = \"%v\"\n", spec.ControlPlaneEndpoint))
	if spec.ETCDConfig.Version != "" {
		buffer.WriteString(fmt.Sprintf("etcd_version = \"%v\"\n", spec.ETCDConfig.Version))
	}
	if spec.OIDCConfig.IssuerURL != "" {
		buffer.WriteString(fmt.Sprintf("oidc_issuer_url = \"%v\"\n", spec.OIDCConfig.IssuerURL))
	}
	if spec.OIDCConfig.ClientID != "" {
		buffer.WriteString(fmt.Sprintf("oidc_client_id = \"%v\"\n", spec.OIDCConfig.ClientID))
	}
	if spec.OIDCConfig.CAFile != "" {
		buffer.WriteString(fmt.Sprintf("oidc_ca_file = \"%v\"\n", spec.OIDCConfig.CAFile))
	}

	if spec.CRIConfig.Version != "" {
		buffer.WriteString(fmt.Sprintf("cri_version = \"%v\"\n", spec.CRIConfig.Version))
	}
	if spec.CRIConfig.Proxy != "" {
		buffer.WriteString(fmt.Sprintf("cri_proxy = \"%v\"\n", spec.CRIConfig.Proxy))
	}
	if len(spec.CRIConfig.DNS) > 0 {
		buffer.WriteString(fmt.Sprintf("cri_dns = [\"%v\"]\n", strings.Join(spec.CRIConfig.DNS, "\",\"")))
	}
	if len(spec.CRIConfig.Mirrors) > 0 {
		buffer.WriteString(fmt.Sprintf("cri_mirrors = [\"%v\"]\n", strings.Join(spec.CRIConfig.Mirrors, "\",\"")))
	}

	buffer.WriteString(fmt.Sprintf("env = \"%v\"\n", spec.EnvironmentName))
	buffer.WriteString(fmt.Sprintf("datacenter = \"%v\"\n", spec.Config.DatacenterName))
	buffer.WriteString(fmt.Sprintf("vsphere_cluster = \"%v\"\n", spec.Config.Cluster))
	if len(spec.Config.EsxiHost) > 0 {
		buffer.WriteString(fmt.Sprintf("esxihosts = [\"%v\"]\n", strings.Join(spec.Config.EsxiHost, "\",\"")))
	} else {
		buffer.WriteString("esxihosts = []\n")
	}
	buffer.WriteString(fmt.Sprintf("datastore = \"%v\"\n", spec.Config.Datastore))
	buffer.WriteString(fmt.Sprintf("network = \"%v\"\n", spec.NetworkConfig.Name))
	buffer.WriteString(fmt.Sprintf("net_cidr = \"%v\"\n", spec.ClusterCIDR))
	buffer.WriteString(fmt.Sprintf("net_gateway = \"%v\"\n", spec.NetworkConfig.Gateway))
	buffer.WriteString(
		fmt.Sprintf(
			"net_nameservers = [\"%v\"]\n",
			strings.Join(spec.NetworkConfig.Nameservers, "\",\""),
		),
	)
	buffer.WriteString(fmt.Sprintf("net_domain = \"%v\"\n", spec.NetworkConfig.Domain))
	buffer.WriteString(fmt.Sprintf("ip_offset = %v\n", spec.NetworkConfig.IPOffset))
	if len(spec.SSHPublicKey) > 0 {
		buffer.WriteString(fmt.Sprintf("ssh_public_keys = [\"%v\"]\n", strings.Join(spec.SSHPublicKey, "\",\"")))
	} else {
		buffer.WriteString("ssh_public_keys = []\n")
	}
	buffer.WriteString(fmt.Sprintf("kube_lb_count = %v\n", spec.LoadBalancerNode.Count))
	buffer.WriteString(fmt.Sprintf("kube_lb_template = \"%v\"\n", spec.LoadBalancerNode.Template))
	buffer.WriteString(fmt.Sprintf("kube_lb_custom_script_path = \"%v\"\n", spec.LoadBalancerNode.CustomScriptPath))
	buffer.WriteString(fmt.Sprintf("kube_master_count = %v\n", spec.MasterNode.Count))
	buffer.WriteString(fmt.Sprintf("kube_master_cpu = %v\n", spec.MasterNode.CPU))
	buffer.WriteString(fmt.Sprintf("kube_master_mem = %v\n", spec.MasterNode.MemSize))
	buffer.WriteString(fmt.Sprintf("kube_master_disk_size = %v\n", spec.MasterNode.DiskSize))
	buffer.WriteString(fmt.Sprintf("kube_master_template = \"%v\"\n", spec.MasterNode.Template))
	// TODO: restore
	if len(spec.MasterNode.Labels) > 0 {
		var labels []byte
		labels, err = json.Marshal(spec.MasterNode.Labels)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("kube_master_labels = %v\n", string(labels)))
	} else {
		buffer.WriteString("kube_master_labels = {}\n")
	}
	if len(spec.MasterNode.Taints) > 0 {
		buffer.WriteString(
			fmt.Sprintf(
				"kube_master_taints = [\"%v\"]\n",
				strings.Join(spec.MasterNode.Taints, "\",\""),
			),
		)
	} else {
		buffer.WriteString("kube_master_taints = []\n")
	}
	buffer.WriteString(fmt.Sprintf("kube_master_custom_script_path = \"%v\"\n", spec.MasterNode.CustomScriptPath))

	buffer.WriteString(fmt.Sprintf("kube_pod_cidr = \"%v\"\n", spec.ClusterPODCIDR))
	buffer.WriteString(fmt.Sprintf("kube_svc_cidr = \"%v\"\n", spec.ClusterSVCCIDR))

	buffer.WriteString(fmt.Sprintf("kube_infra_count = %v\n", spec.InfraNode.Count))
	buffer.WriteString(fmt.Sprintf("kube_infra_cpu = %v\n", spec.InfraNode.CPU))
	buffer.WriteString(fmt.Sprintf("kube_infra_mem = %v\n", spec.InfraNode.MemSize))
	buffer.WriteString(fmt.Sprintf("kube_infra_disk_size = %v\n", spec.InfraNode.DiskSize))
	buffer.WriteString(fmt.Sprintf("kube_infra_template = \"%v\"\n", spec.InfraNode.Template))
	if len(spec.InfraNode.Labels) > 0 {
		var labels []byte
		labels, err = json.Marshal(spec.InfraNode.Labels)
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("kube_infra_labels = %v\n", string(labels)))
	} else {
		buffer.WriteString("kube_infra_labels = {}\n")
	}
	if len(spec.InfraNode.Taints) > 0 {
		buffer.WriteString(fmt.Sprintf("kube_infra_taints = [\"%v\"]\n", strings.Join(spec.InfraNode.Taints, "\",\"")))
	} else {
		buffer.WriteString("kube_infra_taints = []\n")
	}
	buffer.WriteString(fmt.Sprintf("kube_infra_custom_script_path = \"%v\"\n", spec.InfraNode.CustomScriptPath))

	if len(spec.NodePools) > 0 {
		buffer.WriteString("node_pools = [\n")
		for _, np := range spec.NodePools {
			buffer.WriteString("{\n")
			buffer.WriteString(fmt.Sprintf("role = \"%v\"\n", np.Role))
			buffer.WriteString(fmt.Sprintf("template = \"%v\"\n", np.Template))
			buffer.WriteString(fmt.Sprintf("count = %v\n", np.Count))
			buffer.WriteString(fmt.Sprintf("memory = %v\n", np.MemSize))
			buffer.WriteString(fmt.Sprintf("cpu = %v\n", np.CPU))
			buffer.WriteString(fmt.Sprintf("disk_size = %v\n", np.DiskSize))
			if len(np.Labels) > 0 {
				var labels []byte
				labels, err = json.Marshal(np.Labels)
				if err != nil {
					return err
				}
				buffer.WriteString(fmt.Sprintf("labels = %v\n", string(labels)))
			} else {
				buffer.WriteString("labels = {}\n")
			}
			if len(np.Taints) > 0 {
				buffer.WriteString(fmt.Sprintf("taints = [\"%v\"]\n", strings.Join(np.Taints, "\",\"")))
			} else {
				buffer.WriteString("taints = []\n")
			}
			// TODO: restore
			buffer.WriteString(fmt.Sprintf("custom_script_path = \"%v\"\n", ""))
			buffer.WriteString("},\n")
		}
		buffer.WriteString("]\n")
	}

	err = ioutil.WriteFile(fmt.Sprintf("%v/vsphere.tfvars", e.terraform.WorkingDir()), buffer.Bytes(), 0600)
	if err != nil {
		return err
	}
	err = e.terraform.FormatWrite(
		context.Background(),
		tfexec.Dir(fmt.Sprintf("%v/vsphere.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		return err
	}
	return nil
}

// New instantiates a new vSphere provisioner
func New(config *configuration.Configuration) *VSphere {
	b := packr.New("vsphereCluster", projectPath)
	return &VSphere{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (e *VSphere) SetTerraformExecutor(tf *tfexec.Terraform) {
	e.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (e *VSphere) TerraformExecutor() (tf *tfexec.Terraform) {
	return e.terraform
}

// Box returns the box that has the files as binary data
func (e VSphere) Box() *packr.Box {
	return e.box
}

// TODO: find Terraform files
// TODO: find Ansible files
// TODO: rename method TerraformFiles() in FilesToBudle()

// TerraformFiles returns the list of files conforming the terraform project
func (e VSphere) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	// find . -type f -follow -print
	return []string{
		"output.tf",
		"main.tf",
		"variables.tf",
		"provision/ansible.cfg",
		"provision/all-in-one.yml",
		"furyagent/furyagent.yml",
	}
}

// Prepare the environment before running anything
func (e VSphere) Prepare() error {
	if err := createPKI(e.terraform.WorkingDir()); err != nil {
		return fmt.Errorf("error creating PKI: %v", err)
	}

	if err := downloadAnsibleRoles(e.terraform.WorkingDir()); err != nil {
		return fmt.Errorf("error downloading Ansible roles: %v", err)
	}

	return nil
}

func downloadAnsibleRoles(workingDirectory string) error {
	p_netrc := os.Getenv("NETRC")
	defer os.Setenv("NETRC", p_netrc)

	netrcpath := filepath.Join(workingDirectory, "configuration/.netrc")
	logrus.Infof("Configuring the NETRC environment variable: %v", netrcpath)
	os.Setenv("NETRC", netrcpath)

	downloadPath := filepath.Join(workingDirectory, "provision/roles")
	logrus.Infof("Ansible roles download path: %v", downloadPath)
	if err := os.Mkdir(downloadPath, 0755); err != nil {
		return err
	}

	client := &getter.Client{
		Src:  "https://github.com/sighupio/furyctl-provisioners/archive/refs/tags/v0.7.0.zip//furyctl-provisioners-0.7.0/roles",
		Dst:  downloadPath,
		Pwd:  workingDirectory,
		Mode: getter.ClientModeAny,
	}

	return client.Get()
}

// Plan runs a dry run execution
func (e VSphere) Plan() (err error) {
	logrus.Info("[DRYRUN] Updating VSphere Cluster project")
	// TODO: give the name of the file
	err = e.createVarFile()
	if err != nil {
		return err
	}
	var changes bool
	changes, err = e.terraform.Plan(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/vsphere.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		logrus.Fatalf("[DRYRUN] Something went wrong while updating vsphere. %v", err)
		return err
	}
	if changes {
		logrus.Warn("[DRYRUN] Something changed along the time. Remove dryrun option to apply the desired state")
	} else {
		logrus.Info("[DRYRUN] Everything is up to date")
	}

	logrus.Info("[DRYRUN] VSphere Updated")
	return nil
}

// Update runs terraform apply in the project
func (e VSphere) Update() (string, error) {
	logrus.Info("Updating VSphere project")
	err := e.createVarFile()
	if err != nil {
		return "", err
	}
	err = e.terraform.Apply(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/vsphere.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		logrus.Fatalf("Something went wrong while updating vsphere. %v", err)
		return "", err
	}

	var output map[string]tfexec.OutputMeta
	output, err = e.terraform.Output(context.Background())
	if err != nil {
		logrus.Error("Can not get output values")
		return "", err
	}

	var ansibleInventory, haproxyConfig string
	err = json.Unmarshal(output["ansible_inventory"].Value, &ansibleInventory)
	if err != nil {
		logrus.Error("Can not get `ansible_inventory` value")
		return "", err
	}
	err = json.Unmarshal(output["haproxy_config"].Value, &haproxyConfig)
	if err != nil {
		logrus.Error("Can not get `haproxy_config` value")
		return "", err
	}

	filePath := filepath.Join(e.terraform.WorkingDir(), "provision/hosts.ini")
	err = ioutil.WriteFile(filePath, []byte(ansibleInventory), 0644)
	if err != nil {
		return "", err
	}

	filePath = filepath.Join(e.terraform.WorkingDir(), "provision/haproxy.cfg")
	err = ioutil.WriteFile(filePath, []byte(haproxyConfig), 0644)
	if err != nil {
		return "", err
	}

	kubeconfig, err := runAnsiblePlaybook(
		filepath.Join(e.terraform.WorkingDir(), "provision"),
		filepath.Join(e.terraform.WorkingDir(), "logs"),
	)
	log.Info("VSphere Updated")
	return kubeconfig, err
}

// Destroy runs terraform destroy in the project
func (e VSphere) Destroy() (err error) {
	logrus.Info("Destroying VSphere project")
	err = e.createVarFile()
	if err != nil {
		return err
	}
	err = e.terraform.Destroy(
		context.Background(),
		tfexec.VarFile(fmt.Sprintf("%v/vsphere.tfvars", e.terraform.WorkingDir())),
	)
	if err != nil {
		logrus.Fatalf("Something went wrong while destroying VSphere cluster project. %v", err)
		return err
	}
	logrus.Info("VSphere destroyed")
	return nil
}

func createPKI(workingDirectory string) error {
	source := fmt.Sprintf(
		"https://github.com/sighupio/furyagent/releases/download/v0.3.0/furyagent-%s-%s",
		runtime.GOOS,
		runtime.GOARCH,
	)
	downloadPath := filepath.Join(workingDirectory, "furyagent")

	log.Infof("Download furyagent: %v", downloadPath)

	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return err
	}

	client := &getter.Client{
		Src:  source,
		Dst:  downloadPath,
		Pwd:  workingDirectory,
		Mode: getter.ClientModeAny,
	}
	if err := client.Get(); err != nil {
		return err
	}

	tokens := strings.Split(source, "/")
	downloadedExecutableName := tokens[len(tokens)-1]
	wantedExecutableName := "furyagent"

	if err := os.Rename(
		filepath.Join(downloadPath, downloadedExecutableName),
		filepath.Join(downloadPath, wantedExecutableName),
	); err != nil {
		logrus.Fatal(err)
	}

	os.Chmod(filepath.Join(downloadPath, wantedExecutableName), 0755)

	cmd := exec.Command("./furyagent", "init", "master")
	cmd.Dir = downloadPath
	out, err := cmd.Output()
	if err != nil {
		logrus.Debugf("%s", out)
		logrus.Fatal(err)
	}

	cmd = exec.Command("./furyagent", "init", "etcd")
	cmd.Dir = downloadPath
	out, err = cmd.Output()
	if err != nil {
		logrus.Debugf("%s", out)
		logrus.Fatal(err)
	}

	return nil
}

func runAnsiblePlaybook(workingDir string, logDir string) (string, error) {
	logrus.Infof("Run Ansible playbook in : %v", workingDir)

	// TODO: Get the debug flag from the CLI to output both to a file and stdout
	// open the log file for writing
	logFilePath := filepath.Join(logDir, "ansible.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		logrus.Errorf("Can not open the log file %v", logFilePath)
		return "", err
	}
	defer logFile.Close()

	cmd := exec.Command("ansible", "--version")
	cmd.Dir = workingDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Run()
	if err != nil {
		logrus.Debug("Please make sure you have Ansible installed in this machine")
		logrus.Fatal(err)
		return "", err
	}

	cmd = exec.Command("ansible-playbook", "all-in-one.yml")
	cmd.Dir = workingDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Start()
	if err != nil {
		logrus.Fatal(err)
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		logrus.Fatal(err)
		return "", err
	}

	dat, err := ioutil.ReadFile(filepath.Join(workingDir, "../secrets/users/admin.conf"))
	return string(dat), err
}
