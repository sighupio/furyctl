package configuration

import (
	"errors"
	"fmt"
	"io/ioutil"

	bootstrapcfg "github.com/sighupio/furyctl/internal/bootstrap/configuration"
	clustercfg "github.com/sighupio/furyctl/internal/cluster/configuration"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// TerraformExecutor represents the terraform executor configuration to be used
type TerraformExecutor struct {
	// Local Path
	Path string `yaml:"path"`
	// Version to download
	Version string `yaml:"version"`
	// StateConfiguration configures the terraform state to use
	StateConfiguration StateConfiguration `yaml:"state"`
}

// StateConfiguration represents the terraform state configuration to be used
type StateConfiguration struct {
	Backend string            `yaml:"backend"`
	Config  map[string]string `yaml:"config"`
}

// Configuration represents the base of the configuration file
type Configuration struct {
	Kind        string            `yaml:"kind"`
	Metadata    Metadata          `yaml:"metadata"`
	Spec        interface{}       `yaml:"spec"`
	Executor    TerraformExecutor `yaml:"executor"`
	Provisioner string            `yaml:"provisioner"`
}

// Metadata represents a set of metadata information to be used while performing operations
type Metadata struct {
	Name   string                 `yaml:"name"`
	Labels map[string]interface{} `yaml:"labels"`
}

// Parse parses a yaml configuration file (path) returning the parsed configuration file as a Configuration struct
func Parse(path string) (*Configuration, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return nil, err
	}
	baseConfig := &Configuration{}
	err = yaml.Unmarshal(content, &baseConfig)
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return nil, err
	}

	switch {
	case baseConfig.Kind == "Cluster":
		err = clusterParser(baseConfig)
		if err != nil {
			return nil, err
		}
		return baseConfig, nil
	case baseConfig.Kind == "Bootstrap":
		err = bootstrapParser(baseConfig)
		if err != nil {
			return nil, err
		}
		return baseConfig, nil
	default:
		log.Errorf("Error parsing the configuration file. Parser not found for %v kind", baseConfig.Kind)
		return nil, fmt.Errorf("Parser not found for %v kind", baseConfig.Kind)
	}
}

func clusterParser(config *Configuration) (err error) {
	provisioner := config.Provisioner
	log.Debugf("provisioner: %v", provisioner)
	specBytes, err := yaml.Marshal(config.Spec)
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	switch {
	case provisioner == "eks":
		eksSpec := clustercfg.EKS{}
		err = yaml.Unmarshal(specBytes, &eksSpec)
		if err != nil {
			log.Errorf("error parsing configuration file: %v", err)
			return err
		}
		config.Spec = eksSpec
		return nil
	default:
		log.Error("Error parsing the configuration file. Provisioner not found")
		return errors.New("Cluster provisioner not found")
	}
}

func bootstrapParser(config *Configuration) (err error) {
	provisioner := config.Provisioner
	log.Debugf("provisioner: %v", provisioner)
	specBytes, err := yaml.Marshal(config.Spec)
	if err != nil {
		log.Errorf("error parsing configuration file: %v", err)
		return err
	}
	switch {
	case provisioner == "aws":
		awsSpec := bootstrapcfg.AWS{}
		err = yaml.Unmarshal(specBytes, &awsSpec)
		if err != nil {
			log.Errorf("error parsing configuration file: %v", err)
			return err
		}
		config.Spec = awsSpec
		return nil
	default:
		log.Error("Error parsing the configuration file. Provisioner not found")
		return errors.New("Bootstrap provisioner not found")
	}
}
