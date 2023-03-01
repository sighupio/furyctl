// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/internal/tool/furyagent"
	"github.com/sighupio/furyctl/internal/tool/openvpn"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
)

var (
	ErrAutoConnectWithoutVpn = errors.New("autoconnect is not supported without a VPN configuration")
	ErrReadStdin             = errors.New("error reading from stdin")
)

type VpnConnector struct {
	clusterName string
	certDir     string
	autoconnect bool
	skip        bool
	config      *private.SpecInfrastructureVpcVpn
	ovRunner    *openvpn.Runner
	faRunner    *furyagent.Runner
}

func NewVpnConnector(
	clusterName,
	certDir,
	binPath,
	faVersion string,
	autoconnect,
	skip bool,
	config *private.SpecInfrastructureVpcVpn,
) *VpnConnector {
	executor := execx.NewStdExecutor()

	return &VpnConnector{
		clusterName: clusterName,
		certDir:     certDir,
		autoconnect: autoconnect,
		skip:        skip,
		config:      config,
		ovRunner: openvpn.NewRunner(executor, openvpn.Paths{
			WorkDir: certDir,
			Openvpn: "openvpn",
		}),
		faRunner: furyagent.NewRunner(executor, furyagent.Paths{
			Furyagent: path.Join(binPath, "furyagent", faVersion, "furyagent"),
			WorkDir:   certDir,
		}),
	}
}

func (v *VpnConnector) Connect() error {
	if v.autoconnect {
		if !v.IsConfigured() {
			return ErrAutoConnectWithoutVpn
		}

		return v.startOpenVPN()
	}

	if !v.skip {
		return v.prompt()
	}

	return nil
}

func (v *VpnConnector) GenerateCertificates() error {
	clientName, err := v.ClientName()
	if err != nil {
		return err
	}

	if err := v.faRunner.ConfigOpenvpnClient(clientName); err != nil {
		return fmt.Errorf("error configuring openvpn client: %w", err)
	}

	if err := v.copyOpenvpnToWorkDir(clientName); err != nil {
		return fmt.Errorf("error copying openvpn file to workdir: %w", err)
	}

	return nil
}

func (v *VpnConnector) IsConfigured() bool {
	vpn := v.config
	if vpn == nil {
		return false
	}

	instances := v.config.Instances
	if instances == nil {
		return true
	}

	return *instances > 0
}

func (v *VpnConnector) ClientName() (string, error) {
	whoamiResp, err := exec.Command("whoami").Output()
	if err != nil {
		return "", fmt.Errorf("error getting current user: %w", err)
	}

	whoami := strings.TrimSpace(string(whoamiResp))

	return fmt.Sprintf("%s-%s", v.clusterName, whoami), nil
}

func (v *VpnConnector) copyOpenvpnToWorkDir(clientName string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	ovpnFileName := fmt.Sprintf("%s.ovpn", clientName)

	ovpnPath, err := filepath.Abs(path.Join(v.certDir, ovpnFileName))
	if err != nil {
		return fmt.Errorf("error getting ovpn absolute path: %w", err)
	}

	ovpnFile, err := os.ReadFile(ovpnPath)
	if err != nil {
		return fmt.Errorf("error reading ovpn file: %w", err)
	}

	err = os.WriteFile(path.Join(currentDir, ovpnFileName), ovpnFile, iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing ovpn file: %w", err)
	}

	return nil
}

func (v *VpnConnector) startOpenVPN() error {
	connectMsg := "Connecting to VPN"

	isRoot, err := osx.IsRoot()
	if err != nil {
		return fmt.Errorf("error while checking if user is root: %w", err)
	}

	clientName, err := v.ClientName()
	if err != nil {
		return fmt.Errorf("error getting client name: %w", err)
	}

	if !isRoot {
		connectMsg = fmt.Sprintf("%s, you will be asked for your SUDO password", connectMsg)
	}

	logrus.Infof("%s...", connectMsg)

	if err := v.ovRunner.Connect(clientName); err != nil {
		return fmt.Errorf("error connecting to VPN: %w", err)
	}

	return nil
}

func (v *VpnConnector) prompt() error {
	connectMsg := "Please connect to the VPN before continuing"

	clientName, err := v.ClientName()
	if err != nil {
		return fmt.Errorf("error getting client name: %w", err)
	}

	certPath := filepath.Join(v.certDir, clientName)

	if v.IsConfigured() {
		connectMsg = fmt.Sprintf(
			"%s, you can find the configuration file in %s.\nPress enter to continue",
			connectMsg,
			certPath,
		)
	}

	logrus.Info(connectMsg)

	logrus.Info("Press enter to continue...")

	if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		return fmt.Errorf("%w: %v", ErrReadStdin, err)
	}

	return nil
}
