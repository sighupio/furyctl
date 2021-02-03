package cluster

import (
	"context"
	"encoding/json"
	"errors"
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
var clusterProjectDefaultSubDirs = []string{"logs", "configuration", "output", "credentials", "bin"}

// Cluster Represents the possible actions that can be made via CLI after some simple validations
type Cluster struct {
	options *Options
	s       *spinner.Spinner

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

// New builds a Cluster object with some configurations using Options
func New(opts *Options) (c *Cluster, err error) {
	// Grab the right provisioner
	p, err := provisioners.Get(*opts.ProvisionerConfiguration)
	if err != nil {
		log.Errorf("error creating the cluster instance while adquiring the right provisioner: %v", err)
		return nil, err
	}
	if p.Enterprise() && opts.TerraformOpts.GitHubToken == "" {
		errorMsg := fmt.Sprintf("error creating the cluster instance. The %v provisioner is an enterprise feature and requires a valid GitHub token. Contact sales@sighup.io", opts.ProvisionerConfiguration.Provisioner)
		log.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}
	c = &Cluster{
		s:           opts.Spin,
		options:     opts,
		project:     opts.Project,
		provisioner: &p,
	}

	c.options.TerraformOpts.Version = c.options.ProvisionerConfiguration.Executor.Version
	c.options.TerraformOpts.BinaryPath = c.options.ProvisionerConfiguration.Executor.Path
	c.options.TerraformOpts.LogDir = "logs"
	c.options.TerraformOpts.ConfigDir = "configuration"
	if opts.ProvisionerConfiguration.Executor.StateConfiguration.Backend == "" { //The default should be a local file
		opts.ProvisionerConfiguration.Executor.StateConfiguration.Backend = "local"
	}
	c.options.TerraformOpts.Backend = opts.ProvisionerConfiguration.Executor.StateConfiguration.Backend
	c.options.TerraformOpts.BackendConfig = opts.ProvisionerConfiguration.Executor.StateConfiguration.Config

	return c, nil
}

// Init intializes a project directory with all files (terraform project, subdirectories...) running terraform init on it
func (c *Cluster) Init() (err error) {
	// Project structure
	c.s.Stop()
	c.s.Suffix = " Creating project structure"
	c.s.Start()
	err = c.project.CreateSubDirs(clusterProjectDefaultSubDirs)
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

func (c *Cluster) postInit() {
	prov := *c.provisioner
	fmt.Printf(`%v
[FURYCTL]

Init phase completed.

Project directory: %v
Terraform logs: %v/logs/terraform.logs

Everything ready to create the infrastructure; execute:

$ furyctl cluster update

`, prov.InitMessage(), c.project.Path, c.project.Path)
}

func (c *Cluster) postUpdate() {
	proj := *c.project
	prov := *c.provisioner
	fmt.Printf(`%v
[FURYCTL]
Update phase completed.

Project directory: %v
Terraform logs: %v/logs/terraform.logs
Output file: %v/output/output.json

Everything is up to date.
Ready to update or destroy the infrastructure; execute:

$ furyctl cluster update
or
$ furyctl cluster destroy

`, prov.UpdateMessage(), proj.Path, proj.Path, proj.Path)
}

func (c *Cluster) postPlan() {
	proj := *c.project
	fmt.Printf(`[FURYCTL]
Update (dryrun) phase completed.
Discover the upcoming changes in the terraform log file.

Project directory: %v
Terraform logs: %v/logs/terraform.logs

Ready to update or destroy the infrastructure; execute:

$ furyctl cluster update
or
$ furyctl cluster destroy

`, proj.Path, proj.Path)
}

func (c *Cluster) postDestroy() {
	prov := *c.provisioner
	proj := *c.project
	fmt.Printf(`%v
[FURYCTL]
Destroy phase completed.

Project directory: %v
Terraform logs: %v/logs/terraform.logs

`, prov.DestroyMessage(), proj.Path, proj.Path)
}

// Update updates the cluster (terraform apply)
func (c *Cluster) Update(dryrun bool) (err error) {
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

	if !dryrun {
		c.s.Suffix = " Applying terraform project"
		c.s.Start()
		err = prov.Update()
		if err != nil {
			log.Errorf("Error while updating the cluster. Take a look to the logs. %v", err)
			return err
		}
		c.s.Stop()
		c.s.Suffix = " Saving outputs"
		c.s.Start()
		output, err := c.output()
		if err != nil {
			log.Errorf("Error while getting the output with the cluster data: %v", err)
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
	} else {
		c.s.Suffix = " [DRYRUN] Applying terraform project"
		c.s.Start()
		err = prov.Plan()
		if err != nil {
			log.Errorf("[DRYRUN] Error while updating the cluster. Take a look to the logs. %v", err)
			return err
		}
		c.s.Stop()
		proj := *c.project
		log.Infof("[DRYRUN] Discover the resulting plan in the %v/logs/terraform.logs file", proj.Path)
		c.postPlan()
	}
	return nil
}

// Destroy destroys the cluster (terraform destroy)
func (c *Cluster) Destroy() (err error) {
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
		log.Errorf("Error while destroying the cluster. Take a look to the logs. %v", err)
		return err
	}
	c.s.Stop()
	c.postDestroy()
	return nil
}

// installs/copy files from the provisioner to the working dir
func (c *Cluster) installProvisionerTerraformFiles() (err error) {
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
func (c *Cluster) initTerraformExecutor() (err error) {
	tf := &tfexec.Terraform{}

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

// Output gathers the Output in form of binary data
func (c *Cluster) output() ([]byte, error) {
	prov := *c.provisioner
	log.Info("Gathering output file as json")
	var output map[string]tfexec.OutputMeta
	output, err := prov.TerraformExecutor().Output(context.Background())
	if err != nil {
		log.Fatalf("Error while getting project output: %v", err)
		return nil, err
	}
	return json.MarshalIndent(output, "", "    ")
}
