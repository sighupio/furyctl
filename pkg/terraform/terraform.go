package terraform

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sighupio/furyctl/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type TerraformOptions struct {
	Version    string
	BinaryPath string

	Backend           string
	BackendConfigPath string

	WorkingDir string
	ConfigDir  string

	LogDir string
	Debug  bool
}

func NewExecutor(opts TerraformOptions) (tf *tfexec.Terraform, err error) {
	err = validateTerraformBinaryOrVersion(opts)
	if err != nil {
		return nil, err
	}
	err = validateBackendOptions(opts)
	if err != nil {
		return nil, err
	}
	tfBinary, err := ensure(opts.BinaryPath, opts.Version)
	if err != nil {
		return nil, err
	}
	tf, err = tfexec.NewTerraform(opts.WorkingDir, tfBinary)
	if err != nil {
		return nil, err
	}
	err = configureLogger(tf, opts.WorkingDir, opts.LogDir, opts.Debug)
	if err != nil {
		return nil, err
	}
	err = configureBackend(opts.WorkingDir, opts.Backend, opts.BackendConfigPath, opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	return tf, err
}

func validateTerraformBinaryOrVersion(opts TerraformOptions) (err error) {
	if opts.BinaryPath != "" && opts.Version != "" {
		log.Errorf("terraform binary and terraform version can not be used together")
		return errors.New("terraform binary and terraform version can not be used together")
	}
	return nil
}

func validateBackendOptions(opts TerraformOptions) (err error) {
	if opts.Backend != "local" && opts.BackendConfigPath == "" {
		log.Errorf("backend %v requires a backend configuration path", opts.Backend)
		return fmt.Errorf("backend %v requires a backend configuration path", opts.Backend)
	}
	return nil
}

func configureLogger(tf *tfexec.Terraform, workingDir string, logDir string, debug bool) (err error) {
	logFile, err := os.Create(fmt.Sprintf("%v/%v/terraform.logs", workingDir, logDir))
	tf.SetLogger(log.StandardLogger())
	c := &tfwriter{
		logfile: logFile,
		debug:   debug,
	}
	if err != nil {
		log.Errorf("Can not init log file. %v", err)
		return err
	}
	tf.SetStdout(c)
	tf.SetStderr(c)
	return nil
}

func configureBackend(workingDir string, backend string, backendConfigPath string, configDir string) (err error) {
	err = createBackendFile(workingDir, backend)
	if err != nil {
		return err
	}
	if backendConfigPath != "" {
		err = copyBackendConfigFile(workingDir, backendConfigPath, configDir)
		if err != nil {
			return err
		}
	} else {
		err = createEmptyBackendConfigFile(workingDir, configDir)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateBackendFile creates the backend.tf terraform file with the backend configuration choosen
func createBackendFile(path string, backend string) (err error) {
	backendFileContent := fmt.Sprintf(`terraform {
  backend "%v" {}
}`, backend)
	return ioutil.WriteFile(fmt.Sprintf("%v/backend.tf", path), []byte(backendFileContent), os.FileMode(0644))
}

// CopyBackendConfigFile copies the backend configuration file provided by the user to the config directory
func copyBackendConfigFile(path string, backendConfiguration string, configDir string) (err error) {
	dst := fmt.Sprintf("%v/%v/backend.conf", path, configDir)
	_, err = utils.CopyFile(backendConfiguration, dst)
	if err != nil {
		log.Errorf("Error while copying from %v to %v: %v", backendConfiguration, dst, err)
		return err
	}
	return nil
}
func createEmptyBackendConfigFile(path string, configDir string) (err error) {
	dst := fmt.Sprintf("%v/%v/backend.conf", path, configDir)
	err = ioutil.WriteFile(dst, []byte(""), os.FileMode(0644))
	if err != nil {
		log.Errorf("Error while creating an empty backend configuration file")
		return err
	}
	return nil
}

type tfwriter struct {
	logfile *os.File
	debug   bool
}

func (c *tfwriter) Write(data []byte) (n int, err error) {
	c.logfile.Write(data)
	if c.debug {
		fmt.Print(string(data))
	}
	return len(data), nil
}
