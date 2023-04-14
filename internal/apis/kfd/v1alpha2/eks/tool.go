// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"

	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrOpenVPNNotInstalled = fmt.Errorf("openvpn is not installed")

type ExtraToolsValidator struct {
	executor    execx.Executor
	autoConnect bool
}

func NewExtraToolsValidator(executor execx.Executor, autoConnect bool) *ExtraToolsValidator {
	return &ExtraToolsValidator{
		executor:    executor,
		autoConnect: autoConnect,
	}
}

func (x *ExtraToolsValidator) Validate(confPath string) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	furyctlConf, err := yamlx.FromFileV3[private.EksclusterKfdV1Alpha2](confPath)
	if err != nil {
		return oks, append(errs, err)
	}

	if err := x.openVPN(furyctlConf); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "openvpn")
	}

	return oks, errs
}

func (x *ExtraToolsValidator) openVPN(conf private.EksclusterKfdV1Alpha2) error {
	if conf.Spec.Infrastructure != nil &&
		conf.Spec.Infrastructure.Vpn != nil && (conf.Spec.Infrastructure.Vpn.Instances == nil ||
		conf.Spec.Infrastructure.Vpn.Instances != nil &&
			*conf.Spec.Infrastructure.Vpn.Instances > 0) &&
		x.autoConnect {
		oRunner := openvpn.NewRunner(x.executor, openvpn.Paths{
			Openvpn: "openvpn",
		})

		if _, err := oRunner.Version(); err != nil {
			return ErrOpenVPNNotInstalled
		}
	}

	return nil
}
