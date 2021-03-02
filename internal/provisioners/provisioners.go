// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package provisioners

import (
	"errors"
	"fmt"

	"github.com/gobuffalo/packr/v2"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sighupio/furyctl/internal/bootstrap/provisioners/aws"
	"github.com/sighupio/furyctl/internal/bootstrap/provisioners/gcp"
	"github.com/sighupio/furyctl/internal/cluster/provisioners/eks"
	"github.com/sighupio/furyctl/internal/cluster/provisioners/gke"
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
	case config.Provisioner == "eks":
		return eks.New(&config), nil
	case config.Provisioner == "gke":
		return gke.New(&config), nil
	default:
		log.Error("Provisioner not found")
		return nil, errors.New("Provisioner not found")
	}
}
func getBootstrapProvisioner(config configuration.Configuration) (Provisioner, error) {
	switch {
	case config.Provisioner == "aws":
		return aws.New(&config), nil
	case config.Provisioner == "gcp":
		return gcp.New(&config), nil
	default:
		log.Error("Provisioner not found")
		return nil, errors.New("Provisioner not found")
	}
}
