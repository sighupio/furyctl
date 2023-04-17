// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/schema/private"
	"github.com/sighupio/furyctl/internal/tool/awscli"
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
	config      *private.SpecInfrastructureVpn
	ovRunner    *openvpn.Runner
	faRunner    *furyagent.Runner
	awsRunner   *awscli.Runner
	workDir     string
}

func NewVpnConnector(
	clusterName,
	certDir,
	binPath,
	faVersion string,
	autoconnect,
	skip bool,
	config *private.SpecInfrastructureVpn,
) (*VpnConnector, error) {
	executor := execx.NewStdExecutor()

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current working directory: %w", err)
	}

	return &VpnConnector{
		clusterName: clusterName,
		certDir:     certDir,
		autoconnect: autoconnect,
		skip:        skip,
		config:      config,
		ovRunner: openvpn.NewRunner(executor, openvpn.Paths{
			WorkDir: wd,
			Openvpn: "openvpn",
		}),
		faRunner: furyagent.NewRunner(executor, furyagent.Paths{
			Furyagent: path.Join(binPath, "furyagent", faVersion, "furyagent"),
			WorkDir:   certDir,
		}),
		awsRunner: awscli.NewRunner(
			execx.NewStdExecutor(),
			awscli.Paths{
				Awscli:  "aws",
				WorkDir: certDir,
			},
		),
		workDir: wd,
	}, nil
}

func (v *VpnConnector) Connect() error {
	if v.autoconnect {
		if !v.IsConfigured() {
			return ErrAutoConnectWithoutVpn
		}

		vpn, pid, err := v.checkExistingOpenVPN()
		if err != nil {
			return err
		}

		if vpn {
			if err := v.promptAutoConnect(pid); err != nil {
				return err
			}
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

	bucketName, err := v.getFuryAgentBucketName()
	if err != nil {
		return err
	}

	opvnCertPath := filepath.Join(v.certDir, fmt.Sprintf("%s.ovpn", clientName))

	if _, err := os.Stat(opvnCertPath); os.IsNotExist(err) {
		if err := v.assertOldClientCertificateExists(bucketName, clientName); err == nil {
			c, err := v.backupOldClientCertificate(bucketName, clientName)
			if err != nil {
				return err
			}

			if _, err := v.faRunner.ConfigOpenvpnClient(c, "--revoke"); err != nil {
				return fmt.Errorf("error configuring openvpn client: %w", err)
			}
		}

		out, err := v.faRunner.ConfigOpenvpnClient(clientName)
		if err != nil {
			return fmt.Errorf("error configuring openvpn client: %w", err)
		}

		if err := v.writeOVPNFileToDisk(clientName, out.Bytes()); err != nil {
			return err
		}

		if err := v.copyOpenvpnToWorkDir(clientName); err != nil {
			return fmt.Errorf("error copying openvpn file to workdir: %w", err)
		}
	}

	return nil
}

func (v *VpnConnector) getFuryAgentBucketName() (string, error) {
	faCfg, err := furyagent.ParseConfig(filepath.Join(v.certDir, "furyagent.yml"))
	if err != nil {
		return "", fmt.Errorf("error parsing furyagent config: %w", err)
	}

	return faCfg.Storage.BucketName, nil
}

func (v *VpnConnector) assertOldClientCertificateExists(bucketName, certName string) error {
	if _, err := v.awsRunner.S3("ls", fmt.Sprintf("s3://%s/pki/vpn-client/%s.crt", bucketName, certName)); err != nil {
		return fmt.Errorf("error checking if old certificate exists: %w", err)
	}

	return nil
}

func (v *VpnConnector) backupOldClientCertificate(bucketName, certName string) (string, error) {
	u := uuid.New()

	newCertName := fmt.Sprintf("%s-%s-backup", certName, u.String())

	if _, err := v.awsRunner.S3(
		"mv",
		fmt.Sprintf("s3://%s/pki/vpn-client/%s.crt", bucketName, certName),
		fmt.Sprintf("s3://%s/pki/vpn-client/%s.crt", bucketName, newCertName)); err != nil {
		return newCertName, fmt.Errorf("error backing up old certificate: %w", err)
	}

	return newCertName, nil
}

func (v *VpnConnector) writeOVPNFileToDisk(certName string, cert []byte) error {
	err := os.WriteFile(
		filepath.Join(v.faRunner.CmdPath(),
			v.certDir,
			fmt.Sprintf("%s.ovpn", certName)),
		cert,
		iox.FullRWPermAccess,
	)
	if err != nil {
		return fmt.Errorf("error writing openvpn file to disk: %w", err)
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
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("error getting current user: %w", err)
	}

	whoami := strings.TrimSpace(u.Username)

	return fmt.Sprintf("%s-%s", v.clusterName, whoami), nil
}

func (v *VpnConnector) GetKillMessage() (string, error) {
	endVpnMsg := "Please remember to kill the VPN connection when you finish doing operations on the cluster"

	if !v.autoconnect {
		return endVpnMsg, nil
	}

	killMsg := "killall openvpn"

	isRoot, err := osx.IsRoot()
	if err != nil {
		return "", fmt.Errorf("error while checking if user is root: %w", err)
	}

	if !isRoot {
		killMsg = fmt.Sprintf("sudo %s", killMsg)
	}

	return fmt.Sprintf("%s, you can do it with the following command: '%s'", endVpnMsg, killMsg), nil
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

func (*VpnConnector) checkExistingOpenVPN() (bool, int32, error) {
	processes, err := process.Processes()
	if err != nil {
		return false, 0, fmt.Errorf("error getting processes: %w", err)
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			logrus.Warning(err)

			continue
		}

		if name == "openvpn" {
			return true, p.Pid, nil
		}
	}

	return false, 0, nil
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

func (*VpnConnector) promptAutoConnect(pid int32) error {
	logrus.Warnf("There is already a VPN connection process running with PID %d,"+
		" please confirm it is intended to be up before you continue.\n", pid)

	logrus.Info("Press enter to continue or CTRL-C to abort...")

	if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		return fmt.Errorf("%w: %v", ErrReadStdin, err)
	}

	return nil
}

func (v *VpnConnector) prompt() error {
	connectMsg := "Please connect to the VPN before continuing"

	clientName, err := v.ClientName()
	if err != nil {
		return fmt.Errorf("error getting client name: %w", err)
	}

	certPath := filepath.Join(v.workDir, fmt.Sprintf("%s.ovpn", clientName))

	if v.IsConfigured() {
		isRoot, err := osx.IsRoot()
		if err != nil {
			return fmt.Errorf("error while checking if user is root: %w", err)
		}

		vpnConnectCmd := fmt.Sprintf("openvpn --config %s --daemon", certPath)

		if !isRoot {
			vpnConnectCmd = fmt.Sprintf("sudo %s", vpnConnectCmd)
		}

		connectMsg = fmt.Sprintf(
			"%s, you can find the configuration file in %s and connect to the VPN by running the command "+
				"'%s' or using your VPN client of choice.",
			connectMsg,
			certPath,
			vpnConnectCmd,
		)
	}

	logrus.Info(connectMsg)

	logrus.Info("Press enter when you are ready to continue...")

	if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		return fmt.Errorf("%w: %v", ErrReadStdin, err)
	}

	return nil
}
