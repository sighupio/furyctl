// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"errors"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var ErrOpenVPNNotInstalled = errors.New("openvpn is not installed")

type ExtraToolsValidator struct {
	executor execx.Executor
}

func NewExtraToolsValidator(executor execx.Executor) *ExtraToolsValidator {
	return &ExtraToolsValidator{
		executor: executor,
	}
}

func (x *ExtraToolsValidator) Validate(confPath string) ([]string, []error) {
	furyctlConf, err := yamlx.FromFileV3[private.EksclusterKfdV1Alpha2](confPath)
	if err != nil {
		return nil, []error{err}
	}

	return x.validateConf(furyctlConf)
}

// validateConf contains the real validation logic separated from Validate so that tests can
// exercise it against the parsed struct without writing throwaway YAML files.
func (x *ExtraToolsValidator) validateConf(conf private.EksclusterKfdV1Alpha2) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	if !vpnConfigured(conf) {
		return oks, errs
	}

	if err := x.openVPN(); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "openvpn")
	}

	return oks, errs
}

func (x *ExtraToolsValidator) openVPN() error {
	oRunner := openvpn.NewRunner(x.executor, openvpn.Paths{
		Openvpn: "openvpn",
	})

	if _, err := oRunner.Version(); err != nil {
		return ErrOpenVPNNotInstalled
	}

	return nil
}

func vpnConfigured(conf private.EksclusterKfdV1Alpha2) bool {
	return conf.Spec.Infrastructure != nil && conf.Spec.Infrastructure.Vpn.IsConfigured()
}
