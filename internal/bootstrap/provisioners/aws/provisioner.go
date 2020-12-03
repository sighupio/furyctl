package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/terraform-exec/tfexec"
	cfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	"github.com/sighupio/furyctl/internal/configuration"

	log "github.com/sirupsen/logrus"
)

// InitMessage return a custom provisioner message the user will see once the cluster is ready to be updated
func (d *AWS) InitMessage() string {
	return `::AWS::

The provisioner is going to create a well-configured AWS VPC with 
all the requirements set to run a production-grade private EKS cluster.

It creates a VPN server so you can deploy the cluster from this computer
once connected to the VPN server.
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (d *AWS) UpdateMessage() string {
	return `::AWS::

The provisioner is has created all the bootstrap components.
Now you can create a openvpn client configuration using the furyagen cli
`
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (d *AWS) DestroyMessage() string {
	return `
TBD
`
}

// AWS represents a dummy provisioner
type AWS struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

const (
	projectPath = "../../../../data/provisioners/bootstrap/aws"
)

// New instantiates a new AWS provisioner
func New(config *configuration.Configuration) *AWS {
	b := packr.New("AWS", projectPath)
	return &AWS{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (d *AWS) SetTerraformExecutor(tf *tfexec.Terraform) {
	d.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (d *AWS) TerraformExecutor() (tf *tfexec.Terraform) {
	return d.terraform
}

// Box returns the box that has the files as binary data
func (d AWS) Box() *packr.Box {
	return d.box
}

// TerraformFiles returns the list of files conforming the terraform project
func (d AWS) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	// find . -type f -follow -print
	return []string{
		"output.tf",
		"main.tf",
		"variables.tf",
		"modules/vpc-and-vpn/output.tf",
		"modules/vpc-and-vpn/main.tf",
		"modules/vpc-and-vpn/vpn.tf",
		"modules/vpc-and-vpn/variables.tf",
		"modules/vpc-and-vpn/templates/furyagent.yml",
		"modules/vpc-and-vpn/templates/ssh-users.yml",
		"modules/vpc-and-vpn/templates/vpn.yml",
		"modules/vpc-and-vpn/vpc.tf",
	}
}

// Update runs terraform apply in the project
func (d AWS) Update() (err error) {
	log.Info("Updating AWS Bootstrap project")
	spec := d.config.Spec.(cfg.AWS)
	err = d.terraform.Apply(context.Background(),
		tfexec.Var(fmt.Sprintf("name=%v", d.config.Metadata.Name)),
		tfexec.Var(fmt.Sprintf("network_cidr=%v", spec.NetworkCIDR)),
		tfexec.Var(fmt.Sprintf("public_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PublicSubnetsCIDRs, "\",\""))),
		tfexec.Var(fmt.Sprintf("private_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PrivateSubnetsCIDRs, "\",\""))),
		tfexec.Var(fmt.Sprintf("vpn_subnetwork_cidr=%v", spec.VPNSubnetCIDR)),
		tfexec.Var(fmt.Sprintf("vpn_ssh_users=[\"%v\"]", strings.Join(spec.VPNSSHUsers, "\",\""))),
		tfexec.Var(fmt.Sprintf("vpn_operator_cidrs=[\"%v\"]", strings.Join(spec.VPNOperatorCIDRs, "\",\""))),
	)
	if err != nil {
		log.Fatalf("Something went wrong while updating aws. %v", err)
		return err
	}

	log.Info("AWS Updated")
	return nil
}

// Destroy runs terraform destroy in the project
func (d AWS) Destroy() (err error) {
	log.Info("Destroying AWS Bootstrap project")
	spec := d.config.Spec.(cfg.AWS)
	err = d.terraform.Destroy(context.Background(),
		tfexec.Var(fmt.Sprintf("name=%v", d.config.Metadata.Name)),
		tfexec.Var(fmt.Sprintf("network_cidr=%v", spec.NetworkCIDR)),
		tfexec.Var(fmt.Sprintf("public_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PublicSubnetsCIDRs, "\",\""))),
		tfexec.Var(fmt.Sprintf("private_subnetwork_cidrs=[\"%v\"]", strings.Join(spec.PrivateSubnetsCIDRs, "\",\""))),
		tfexec.Var(fmt.Sprintf("vpn_subnetwork_cidr=%v", spec.VPNSubnetCIDR)),
		tfexec.Var(fmt.Sprintf("vpn_ssh_users=[\"%v\"]", strings.Join(spec.VPNSSHUsers, "\",\""))),
		tfexec.Var(fmt.Sprintf("vpn_operator_cidrs=[\"%v\"]", strings.Join(spec.VPNOperatorCIDRs, "\",\""))),
	)
	if err != nil {
		log.Fatalf("Something went wrong while destroying AWS Bootstrap project. %v", err)
		return err
	}
	log.Info("AWS Bootstrap destroyed")
	return nil
}

// Output gathers the Output in form of binary data
func (d AWS) Output() ([]byte, error) {
	log.Info("Gathering aws output file as json")
	var output map[string]tfexec.OutputMeta
	output, err := d.terraform.Output(context.Background())
	if err != nil {
		log.Fatalf("Error while getting project output: %v", err)
		return nil, err
	}
	return json.Marshal(output)
}
