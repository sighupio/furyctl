package provisioners

import (
	"errors"
	"fmt"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sighupio/furyctl/internal/bootstrap/provisioners/aws"
	"github.com/sighupio/furyctl/internal/bootstrap/provisioners/dummy"
	"github.com/sighupio/furyctl/internal/configuration"
	log "github.com/sirupsen/logrus"
)

// Provisioner represents a kubernetes terraform provisioner
type Provisioner interface {
	InitMessage() string
	UpdateMessage() string
	DestroyMessage() string

	SetTerraformExecutor(tf *tfexec.Terraform)
	TerraformExecutor() (tf *tfexec.Terraform)
	TerraformFiles() []string

	Enterprise() bool

	Plan() error
	Update() error
	Destroy() error

	Box() *packr.Box
}

// Get returns an initialized provisioner
func Get(config configuration.Configuration) (Provisioner, error) {
	switch {
	case config.Kind == "Cluster":
		return getClusterProvisioner(config)
	case config.Kind == "Bootstrap":
		return getBootstrapProvisioner(config)
	default:
		log.Errorf("Kind %v not found", config.Kind)
		return nil, fmt.Errorf("Kind %v not found", config.Kind)
	}
}

func getClusterProvisioner(config configuration.Configuration) (Provisioner, error) {
	switch {
	// case config.Provisioner == "aws-simple":
	// 	return awssimple.New(&config), nil
	default:
		log.Error("Provisioner not found")
		return nil, errors.New("Provisioner not found")
	}
}
func getBootstrapProvisioner(config configuration.Configuration) (Provisioner, error) {
	switch {
	case config.Provisioner == "aws":
		return aws.New(&config), nil
	case config.Provisioner == "dummy":
		return dummy.New(&config), nil
	default:
		log.Error("Provisioner not found")
		return nil, errors.New("Provisioner not found")
	}
}
