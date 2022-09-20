// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bootstrap

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
	"github.com/sirupsen/logrus"
)

const initExecutorMessage = " Initializing the terraform executor"

// List of default subdirectories needed to run any provisioner.
var bootstrapProjectDefaultSubDirs = []string{"logs", "configuration", "output", "bin", "secrets"}

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
	TerraformOpts            *terraform.Options
}

// New builds a Bootstrap object with some configurations using Options
func New(opts *Options) (b *Bootstrap, err error) {
	// Grab the right provisioner
	p, err := provisioners.Get(*opts.ProvisionerConfiguration)
	if err != nil {
		logrus.Errorf("error creating the bootstrap instance while adquiring the right provisioner: %v", err)
		return nil, err
	}
	if p.Enterprise() && opts.TerraformOpts.GitHubToken == "" {
		logrus.Warningf("The %v provisioner is an enterprise feature and requires a valid GitHub token", opts.ProvisionerConfiguration.Provisioner)
	}
	b = &Bootstrap{
		s:           opts.Spin,
		options:     opts,
		project:     opts.Project,
		provisioner: &p,
	}
	b.options.TerraformOpts.Version = "0.15.4"
	b.options.TerraformOpts.LogDir = "logs"
	b.options.TerraformOpts.ConfigDir = "configuration"
	if opts.ProvisionerConfiguration.Executor.StateConfiguration.Backend == "" { // The default should be a local file
		opts.ProvisionerConfiguration.Executor.StateConfiguration.Backend = "local"
	}
	b.options.TerraformOpts.Backend = opts.ProvisionerConfiguration.Executor.StateConfiguration.Backend
	b.options.TerraformOpts.BackendConfig = opts.ProvisionerConfiguration.Executor.StateConfiguration.Config

	return b, nil
}

// Init intializes a project directory with all files (terraform project, subdirectories...) running terraform init on it
func (c *Bootstrap) Init(reset bool) (err error) {
	prov := *c.provisioner

	// Enterprise token validation
	if prov.Enterprise() && c.options.TerraformOpts.GitHubToken == "" {
		errorMsg := fmt.Sprintf("error while initiating the bootstap process. The %v provisioner is an enterprise feature and requires a valid GitHub token. Contact sales@sighup.io", c.options.ProvisionerConfiguration.Provisioner)
		logrus.Error(errorMsg)
		return errors.New(errorMsg)
	}

	// Reset the project directory
	if reset {
		logrus.Warn("Cleaning up the workdir")
		err = c.project.Reset()
		if err != nil {
			logrus.Errorf("Error cleaning up the workdir")
			return err
		}
	}

	// Project structure
	c.s.Stop()
	c.s.Suffix = " Creating project structure"
	c.s.Start()
	err = c.project.CreateSubDirs(bootstrapProjectDefaultSubDirs)
	if err != nil {
		logrus.Errorf("error while initializing project subdirectories: %v", err)
		return err
	}

	// .gitignore and .gitattributes
	err = c.createGitFiles()
	if err != nil {
		logrus.Errorf("error while initializing project git files: %v", err)
		return err
	}

	// terraform executor
	c.s.Stop()
	c.s.Suffix = initExecutorMessage
	c.s.Start()
	err = c.initTerraformExecutor()

	if err != nil {
		logrus.Errorf("error while initializing terraform executor: %v", err)
		return err
	}

	// Install the provisioner files into the project structure
	c.s.Stop()
	c.s.Suffix = " Installing provisioner terraform files"
	c.s.Start()
	err = c.installProvisionerTerraformFiles()
	if err != nil {
		logrus.Errorf("error while copying terraform project from the provisioner to the project dir: %v", err)
		return err
	}

	// Init the terraform project
	tf := prov.TerraformExecutor()
	c.s.Stop()
	c.s.Suffix = " Initializing terraform project"
	c.s.Start()

	err = tf.Init(context.Background(), tfexec.Reconfigure(c.options.TerraformOpts.ReconfigureBackend))
	if err != nil {
		logrus.Errorf("error while running terraform init in the project dir: %v", err)
		return err
	}
	c.s.Stop()
	c.postInit()
	return nil
}

func (c *Bootstrap) postInit() {
	prov := *c.provisioner
	fmt.Printf(`%v
[FURYCTL]

Init phase completed.

Project directory: %v
Terraform logs: %v/logs/terraform.logs

Everything ready to create the infrastructure; execute:

$ furyctl bootstrap apply

`, prov.InitMessage(), c.project.Path, c.project.Path)
}

func (c *Bootstrap) postUpdate() {
	proj := *c.project
	prov := *c.provisioner
	fmt.Printf(`%v
[FURYCTL]
Apply phase completed.

Project directory: %v
Terraform logs: %v/logs/terraform.logs
Output file: %v/output/output.json

Everything is up to date.
Ready to apply or destroy the infrastructure; execute:

$ furyctl bootstrap apply
or
$ furyctl bootstrap destroy

`, prov.UpdateMessage(), proj.Path, proj.Path, proj.Path)
}

func (c *Bootstrap) postPlan() {
	proj := *c.project
	fmt.Printf(`[FURYCTL]
Apply (dryrun) phase completed.
Discover the upcoming changes in the terraform log file.

Project directory: %v
Terraform logs: %v/logs/terraform.logs

Ready to apply or destroy the infrastructure; execute:

$ furyctl bootstrap apply
or
$ furyctl bootstrap destroy

`, proj.Path, proj.Path)
}

func (c *Bootstrap) postDestroy() {
	prov := *c.provisioner
	proj := *c.project
	fmt.Printf(`%v
[FURYCTL]
Destroy phase completed.

Project directory: %v
Terraform logs: %v/logs/terraform.logs

`, prov.DestroyMessage(), proj.Path, proj.Path)
}

// Update updates the bootstrap (terraform apply)
func (c *Bootstrap) Update(dryrun bool) (err error) {
	// Project structure
	c.s.Stop()
	c.s.Suffix = " Updating project structure"
	c.s.Start()
	err = c.project.CreateSubDirs(bootstrapProjectDefaultSubDirs)
	if err != nil {
		logrus.Warnf("error while updating project subdirectories: %v", err)
	}

	// .gitignore and .gitattributes
	err = c.createGitFiles()
	if err != nil {
		logrus.Errorf("error while initializing project git files: %v", err)
		return err
	}

	c.s.Stop()
	c.s.Suffix = initExecutorMessage
	c.s.Start()
	err = c.initTerraformExecutor()
	if err != nil {
		logrus.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	// Install the provisioner files into the project structure
	c.s.Stop()
	c.s.Suffix = " Updating provisioner terraform files"
	c.s.Start()
	err = c.installProvisionerTerraformFiles()
	if err != nil {
		logrus.Warnf("error while copying terraform project from the provisioner to the project dir: %v", err)
	}

	prov := *c.provisioner
	c.s.Stop()

	// Init the terraform project
	tf := prov.TerraformExecutor()
	c.s.Suffix = " Re-Initializing terraform project"
	c.s.Start()

	err = tf.Init(context.Background(), tfexec.Reconfigure(c.options.TerraformOpts.ReconfigureBackend))
	if err != nil {
		logrus.Errorf("error while running terraform init in the project dir: %v", err)
		return err
	}

	if !dryrun {
		c.s.Suffix = " Applying terraform project"
		c.s.Start()
		_, err = prov.Update()
		if err != nil {
			logrus.Errorf("Error while updating the bootstrap. Take a look to the logs. %v", err)
			return err
		}
		c.s.Stop()
		c.s.Suffix = " Saving outputs"
		c.s.Start()
		var output []byte
		output, err = c.output()
		if err != nil {
			logrus.Errorf("Error while getting the output with the bootstrap data: %v", err)
			return err
		}

		proj := *c.project
		err = proj.WriteFile("output/output.json", output)
		if err != nil {
			logrus.Errorf("Error while writting the output.json to the project directory: %v", err)
			return err
		}
		c.s.Stop()
		c.postUpdate()
	} else {
		c.s.Suffix = " [DRYRUN] Applying terraform project"
		c.s.Start()
		err = prov.Plan()
		if err != nil {
			logrus.Errorf("[DRYRUN] Error while updating the bootstrap. Take a look to the logs. %v", err)
			return err
		}
		c.s.Stop()
		proj := *c.project
		logrus.Infof("[DRYRUN] Discover the resulting plan in the %v/logs/terraform.logs file", proj.Path)
		c.postPlan()
	}
	return nil
}

// Destroy destroys the bootstrap (terraform destroy)
func (c *Bootstrap) Destroy() (err error) {
	// Project structure
	c.s.Stop()
	c.s.Suffix = " Updating project structure"
	c.s.Start()
	err = c.project.CreateSubDirs(bootstrapProjectDefaultSubDirs)
	if err != nil {
		logrus.Warnf("error while updating project subdirectories: %v", err)
	}

	// .gitignore and .gitattributes
	err = c.createGitFiles()
	if err != nil {
		logrus.Errorf("error while initializing project git files: %v", err)
		return err
	}

	c.s.Stop()
	c.s.Suffix = initExecutorMessage
	c.s.Start()
	err = c.initTerraformExecutor()
	if err != nil {
		logrus.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	// Install the provisioner files into the project structure
	c.s.Stop()
	c.s.Suffix = " Updating provisioner terraform files"
	c.s.Start()
	err = c.installProvisionerTerraformFiles()
	if err != nil {
		logrus.Warnf("error while copying terraform project from the provisioner to the project dir: %v", err)
	}

	prov := *c.provisioner
	c.s.Stop()

	// Init the terraform project
	tf := prov.TerraformExecutor()
	c.s.Suffix = " Re-Initializing terraform project"
	c.s.Start()

	err = tf.Init(context.Background(), tfexec.Reconfigure(c.options.TerraformOpts.ReconfigureBackend))
	if err != nil {
		logrus.Errorf("error while running terraform init in the project dir: %v", err)
		return err
	}

	c.s.Stop()
	c.s.Suffix = " Destroying terraform project"
	c.s.Start()
	err = prov.Destroy()
	if err != nil {
		logrus.Errorf("Error while destroying the bootstrap. Take a look to the logs. %v", err)
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
			logrus.Errorf("Error while finding the right file in the box: %v", err)
			return err
		}
		err = proj.WriteFile(tfFileName, tfFile)
		if err != nil {
			logrus.Errorf("Error while writing the binary data from the box to the project dir: %v", err)
			return err
		}
	}
	return nil
}

// creates the terraform executor to being used by the bootstrap instance and its provisioner
func (c *Bootstrap) initTerraformExecutor() (err error) {
	tf, err := terraform.NewExecutor(*c.options.TerraformOpts)
	if err != nil {
		logrus.Errorf("Error while initializing the terraform executor: %v", err)
		return err
	}

	// Attach the terraform executor to the provisioner
	prov := *c.provisioner
	prov.SetTerraformExecutor(tf)
	return nil
}

// Output gathers the Output in form of binary data
func (c *Bootstrap) output() ([]byte, error) {
	prov := *c.provisioner
	logrus.Info("Gathering output file as json")
	var output map[string]tfexec.OutputMeta
	output, err := prov.TerraformExecutor().Output(context.Background())
	if err != nil {
		logrus.Fatalf("Error while getting project output: %v", err)
		return nil, err
	}
	return json.MarshalIndent(output, "", "    ")
}

func (c *Bootstrap) createGitFiles() error {
	c.s.Stop()
	c.s.Suffix = " Creating .gitattributes file"
	c.s.Start()
	gitattributes := `*secrets/** filter=git-crypt diff=git-crypt
*output/** filter=git-crypt diff=git-crypt
*logs/** filter=git-crypt diff=git-crypt
*configuration/** filter=git-crypt diff=git-crypt
`
	err := c.project.WriteFile(".gitattributes", []byte(gitattributes))
	if err != nil {
		logrus.Errorf("error while creating .gitattributes: %v", err)
		return err
	}

	c.s.Stop()
	c.s.Suffix = " Creating .gitignore file"
	c.s.Start()
	gitignore := `.terraform
bin
`
	err = c.project.WriteFile(".gitignore", []byte(gitignore))
	if err != nil {
		logrus.Errorf("error while creating .gitignore: %v", err)
		return err
	}

	return nil
}
