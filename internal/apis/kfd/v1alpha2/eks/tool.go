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

type ExtraToolsValidator struct{}

func (x *ExtraToolsValidator) Validate(confPath string) error {
	furyctlConf, err := yamlx.FromFileV3[private.EksclusterKfdV1Alpha2](confPath)
	if err != nil {
		return err
	}

	return x.openVPN(furyctlConf)
}

func (*ExtraToolsValidator) openVPN(conf private.EksclusterKfdV1Alpha2) error {
	executor := execx.NewStdExecutor()

	if conf.Spec.Infrastructure.Vpc != nil &&
		conf.Spec.Infrastructure.Vpc.Vpn != nil {
		oRunner := openvpn.NewRunner(executor, openvpn.Paths{
			Openvpn: "openvpn",
		})

		if _, err := oRunner.Version(); err != nil {
			return ErrOpenVPNNotInstalled
		}
	}

	return nil
}
