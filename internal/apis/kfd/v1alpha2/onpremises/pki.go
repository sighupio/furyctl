// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/clusterpki"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// ErrPkiFolder names the configuration field, so that the message of clusterpki points at it.
var ErrPkiFolder = errors.New(".spec.kubernetes.pkiFolder")

// PKIValidator checks the local PKI folder of an OnPremises configuration.
type PKIValidator struct{}

// ValidatePKI checks that the folder in .spec.kubernetes.pkiFolder holds the CA certificates and keys
// that the kubernetes playbook reads. The etcd and control plane roles read the `etcd` and `master`
// folders, and the playbook copies `master/ca.crt` to the load balancers. Without these files the apply
// fails inside Ansible, after it reaches the nodes.
func (*PKIValidator) ValidatePKI(confPath string) error {
	conf, err := yamlx.FromFileV3[public.OnpremisesKfdV1Alpha2](confPath)
	if err != nil {
		return err
	}

	var value string

	if conf.Spec.Kubernetes.PkiFolder != nil {
		value = *conf.Spec.Kubernetes.PkiFolder
	}

	pkiPath, err := clusterpki.ResolvePath(value, filepath.Dir(confPath))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPkiFolder, err)
	}

	if err := clusterpki.Check(pkiPath); err != nil {
		return fmt.Errorf("%w: %w", ErrPkiFolder, err)
	}

	return nil
}
