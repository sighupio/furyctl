package bootstrap

import (
	"context"
	"fmt"

	"github.com/briandowns/spinner"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sighupio/furyctl/internal/provisioners"
	"github.com/sighupio/furyctl/pkg/terraform"
	log "github.com/sirupsen/logrus"
)

// List of default subdirectories needed to run any provisioner.
var bootstrapProjectDefaultSubDirs = []string{"logs", "configuration"}

// Bootstrap Represents the possible actions that can be made via CLI after some simple validations
type Bootstrap struct {
	bootstrapOptions *Options
	s                *spinner.Spinner

	project     *project.Project
	provisioner *provisioners.Provisioner
}

// Options are valid configuration needed to proceed with the cluster management
type Options struct {
	Spin                     *spinner.Spinner
	Project                  *project.Project
	ProvisionerConfiguration *configuration.Configuration
	TerraformOpts            *terraform.TerraformOptions
}

// New builds a Bootstrap object with some configurations using ClusterOptions
func New(opts *Options) (b *Bootstrap, err error) {
	// Grab the right provisioner
	p, err := provisioners.Get(*opts.ProvisionerConfiguration)
	if err != nil {
		log.Errorf("error creating the bootstrap instance while adquiring the right provisioner: %v", err)
		return nil, err
	}
	b = &Bootstrap{
		s:                opts.Spin,
		bootstrapOptions: opts,
		project:          opts.Project,
		provisioner:      &p,
	}
	return b, nil
}

// Init intializes a project directory with all files (terraform project, subdirectories...) running terraform init on it
func (c *Bootstrap) Init() (err error) {
	// Project structure
	c.s.Stop()
	c.s.Suffix = " Creating project structure"
	c.s.Start()
	err = c.project.CreateSubDirs(bootstrapProjectDefaultSubDirs)
	if err != nil {
		log.Errorf("error while initializing project subdirectories: %v", err)
		return err
	}

	// terraform executor
	c.s.Stop()
	c.s.Suffix = " Initializing the terraform executor"
	c.s.Start()
	err = c.initTerraformExecutor()

	if err != nil {
		log.Errorf("error while initializing terraform executor: %v", err)
		return err
	}

	// Install the provisioner files into the project structure
	c.s.Stop()
	c.s.Suffix = " Installing provisioner terraform files"
	c.s.Start()
	err = c.installProvisionerTerraformFiles()
	if err != nil {
		log.Errorf("error while copying terraform project from the provisioner to the project dir: %v", err)
		return err
	}

	// Init the terraform project
	prov := *c.provisioner
	tf := prov.TerraformExecutor()
	// TODO Improve this init command. hardcoded backend.conf value.
	c.s.Stop()
	c.s.Suffix = " Initializing terraform project"
	c.s.Start()

	err = tf.Init(context.Background(), tfexec.BackendConfig(fmt.Sprintf("%v/backend.conf", c.bootstrapOptions.TerraformOpts.ConfigDir)))
	if err != nil {
		log.Errorf("error while running terraform init in the project dir: %v", err)
		return err
	}
	c.s.Stop()
	// c.postInit()
	return nil
}

// installs/copy files from the provisioner to the working dir
func (c *Bootstrap) installProvisionerTerraformFiles() (err error) {
	proj := *c.project
	prov := *c.provisioner
	b := prov.Box()
	for _, tfFileName := range prov.TerraformFiles() {
		tfFile, err := b.Find(tfFileName)
		if err != nil {
			log.Errorf("Error while finding the right file in the box: %v", err)
			return err
		}
		err = proj.WriteFile(tfFileName, tfFile)
		if err != nil {
			log.Errorf("Error while writing the binary data from the box to the project dir: %v", err)
			return err
		}
	}
	return nil
}

// creates the terraform executor to being used by the cluster instance and its provisioner
func (c *Bootstrap) initTerraformExecutor() (err error) {
	tf := &tfexec.Terraform{}
	// Create the terraform executor
	c.bootstrapOptions.TerraformOpts.LogDir = "logs"
	c.bootstrapOptions.TerraformOpts.ConfigDir = "configuration"

	tf, err = terraform.NewExecutor(*c.bootstrapOptions.TerraformOpts)
	if err != nil {
		log.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	// Attach the terraform executor to the provisioner
	prov := *c.provisioner
	prov.SetTerraformExecutor(tf)
	return nil
}
