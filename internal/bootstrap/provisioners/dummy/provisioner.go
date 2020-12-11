package dummy

import (
	"context"
	"fmt"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/terraform-exec/tfexec"
	dummycfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	"github.com/sighupio/furyctl/internal/configuration"

	log "github.com/sirupsen/logrus"
)

// InitMessage return a custom provisioner message the user will see once the cluster is ready to be updated
func (d *Dummy) InitMessage() string {
	return `
Dummy
`
}

// UpdateMessage return a custom provisioner message the user will see once the cluster is updated
func (d *Dummy) UpdateMessage() string {
	return `
Dummy
`
}

// DestroyMessage return a custom provisioner message the user will see once the cluster is destroyed
func (d *Dummy) DestroyMessage() string {
	return `
Dummy
`
}

// Enterprise return a boolean indicating it is not an enterprise provisioner
func (d *Dummy) Enterprise() bool {
	return false
}

// Dummy represents a dummy provisioner
type Dummy struct {
	terraform *tfexec.Terraform
	box       *packr.Box
	config    *configuration.Configuration
}

const (
	projectPath = "../../../../data/provisioners/bootstrap/dummy"
)

// New instantiates a new Dummy provisioner
func New(config *configuration.Configuration) *Dummy {
	b := packr.New("Dummy", projectPath)
	return &Dummy{
		box:    b,
		config: config,
	}
}

// SetTerraformExecutor adds the terraform executor to this provisioner
func (d *Dummy) SetTerraformExecutor(tf *tfexec.Terraform) {
	d.terraform = tf
}

// TerraformExecutor returns the current terraform executor of this provisioner
func (d *Dummy) TerraformExecutor() (tf *tfexec.Terraform) {
	return d.terraform
}

// Box returns the box that has the files as binary data
func (d Dummy) Box() *packr.Box {
	return d.box
}

// TerraformFiles returns the list of files conforming the terraform project
func (d Dummy) TerraformFiles() []string {
	// TODO understand if it is possible to deduce these values somehow
	return []string{"main.tf"}
}

// Plan runs terraform plan in the project
func (d Dummy) Plan() (err error) {
	log.Info("[DRYRUN] Updating Dummy")
	spec := d.config.Spec.(dummycfg.Dummy)
	changes, err := d.terraform.Plan(context.Background(),
		tfexec.Var(fmt.Sprintf("rsa_bits=%v", spec.RSABits)),
	)
	if err != nil {
		log.Fatalf("[DRYRUN] Something went wrong while updating dummy. %v", err)
		return err
	}
	if changes {
		log.Warn("[DRYRUN] Something changed along the time. Remove dryrun option to apply the desired state")
	} else {
		log.Info("[DRYRUN] Everything is up to date")
	}

	log.Info("[DRYRUN] Dummy Updated")
	return nil
}

// Update runs terraform apply in the project
func (d Dummy) Update() (err error) {
	log.Info("Updating Dummy")
	spec := d.config.Spec.(dummycfg.Dummy)
	err = d.terraform.Apply(context.Background(),
		tfexec.Var(fmt.Sprintf("rsa_bits=%v", spec.RSABits)),
	)
	if err != nil {
		log.Fatalf("Something went wrong while updating dummy. %v", err)
		return err
	}

	log.Info("Dummy Updated")
	return nil
}

// Destroy runs terraform destroy in the project
func (d Dummy) Destroy() (err error) {
	log.Info("Destroying dummy")
	spec := d.config.Spec.(dummycfg.Dummy)
	err = d.terraform.Destroy(context.Background(),
		tfexec.Var(fmt.Sprintf("rsa_bits=%v", spec.RSABits)))
	if err != nil {
		log.Fatalf("Something went wrong while destroying dummy. %v", err)
		return err
	}
	log.Info("Dummy destroyed")
	return nil
}
