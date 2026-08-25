// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/clusterpki"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// ErrPkiPath names the configuration field, so that the message of clusterpki points at it.
var ErrPkiPath = errors.New(".spec.kubernetes.pkiPath")

// PKIValidator checks the local PKI folder of an Immutable configuration.
type PKIValidator struct{}

// ValidatePKI checks that the folder in .spec.kubernetes.pkiPath holds the CA certificates and keys
// that the playbooks read. The infrastructure phase reads `master/ca.crt` for the load balancers, and
// the kubernetes phase reads the `master` and `etcd` folders. Without these files the apply fails
// inside Ansible, after it reaches the nodes.
func (*PKIValidator) ValidatePKI(confPath string) error {
	conf, err := yamlx.FromFileV3[public.ImmutableKfdV1Alpha2](confPath)
	if err != nil {
		return err
	}

	var value string

	if conf.Spec.Kubernetes.PkiPath != nil {
		value = *conf.Spec.Kubernetes.PkiPath
	}

	pkiPath, err := clusterpki.ResolvePath(value, filepath.Dir(confPath))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPkiPath, err)
	}

	if err := clusterpki.Check(pkiPath); err != nil {
		return fmt.Errorf("%w: %w", ErrPkiPath, err)
	}

	return nil
}
