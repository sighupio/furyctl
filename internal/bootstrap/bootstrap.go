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
var bootstrapProjectDefaultSubDirs = []string{"logs", "configuration", "output"}

// Bootstrap Represents the possible actions that can be made via CLI after some simple validations
type Bootstrap struct {
	options *Options
	s       *spinner.Spinner

	project     *project.Project
	provisioner *provisioners.Provisioner
}

// Options are valid configuration needed to proceed with the bootstrap management
type Options struct {
	Spin                     *spinner.Spinner
	Project                  *project.Project
	ProvisionerConfiguration *configuration.Configuration
	TerraformOpts            *terraform.TerraformOptions
}

// New builds a Bootstrap object with some configurations using Options
func New(opts *Options) (b *Bootstrap, err error) {
	// Grab the right provisioner
	p, err := provisioners.Get(*opts.ProvisionerConfiguration)
	if err != nil {
		log.Errorf("error creating the bootstrap instance while adquiring the right provisioner: %v", err)
		return nil, err
	}
	b = &Bootstrap{
		s:           opts.Spin,
		options:     opts,
		project:     opts.Project,
		provisioner: &p,
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

	err = tf.Init(context.Background(), tfexec.BackendConfig(fmt.Sprintf("%v/backend.conf", c.options.TerraformOpts.ConfigDir)))
	if err != nil {
		log.Errorf("error while running terraform init in the project dir: %v", err)
		return err
	}
	c.s.Stop()
	c.postInit()
	return nil
}

func (c *Bootstrap) postInit() {
	prov := *c.provisioner
	fmt.Printf(`%v

::SIGHUP::

The init phase has been completed. 
Take a look to the %v path to discover the source code of the project.

Everything is set to actually create the infrastructure. 

Run furyctl bootstrap update command whenever you want.

`, prov.InitMessage(), c.project.Path)
}

func (c *Bootstrap) postUpdate() {
	proj := *c.project
	prov := *c.provisioner
	fmt.Printf(`%v
::SIGHUP::
The bootstrap project has been created. 
The output file is located at %v/output/output.json
`, prov.UpdateMessage(), proj.Path)
}

func (c *Bootstrap) postDestroy() {
	prov := *c.provisioner
	fmt.Printf(`%v
::SIGHUP::
The bootstrap has been destroyed
`, prov.DestroyMessage())
}

// Update updates the bootstrap (terraform apply)
func (c *Bootstrap) Update() (err error) {
	c.s.Stop()
	c.s.Suffix = " Initializing the terraform executor"
	c.s.Start()
	err = c.initTerraformExecutor()
	if err != nil {
		log.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	prov := *c.provisioner
	c.s.Stop()
	c.s.Suffix = " Applying terraform project"
	c.s.Start()
	err = prov.Update()
	if err != nil {
		log.Errorf("Error while updating the bootstrap. Take a look to the logs. %v", err)
		return err
	}
	c.s.Stop()
	c.s.Suffix = " Saving kubeconfig"
	c.s.Start()
	output, err := prov.Output()
	if err != nil {
		log.Errorf("Error while getting the output with the bootstrap data: %v", err)
		return err
	}

	proj := *c.project
	err = proj.WriteFile("output/output.json", output)
	if err != nil {
		log.Errorf("Error while writting the output.json to the project directory: %v", err)
		return err
	}
	c.s.Stop()
	c.postUpdate()
	return nil
}

// Destroy destroys the bootstrap (terraform destroy)
func (c *Bootstrap) Destroy() (err error) {
	c.s.Stop()
	c.s.Suffix = " Initializing the terraform executor"
	c.s.Start()
	err = c.initTerraformExecutor()
	if err != nil {
		log.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	prov := *c.provisioner
	c.s.Stop()
	c.s.Suffix = " Destroying terraform project"
	c.s.Start()
	err = prov.Destroy()
	if err != nil {
		log.Errorf("Error while destroying the bootstrap. Take a look to the logs. %v", err)
		return err
	}
	c.s.Stop()
	c.postDestroy()
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

// creates the terraform executor to being used by the bootstrap instance and its provisioner
func (c *Bootstrap) initTerraformExecutor() (err error) {
	tf := &tfexec.Terraform{}
	// Create the terraform executor
	c.options.TerraformOpts.LogDir = "logs"
	c.options.TerraformOpts.ConfigDir = "configuration"

	tf, err = terraform.NewExecutor(*c.options.TerraformOpts)
	if err != nil {
		log.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	// Attach the terraform executor to the provisioner
	prov := *c.provisioner
	prov.SetTerraformExecutor(tf)
	return nil
}
