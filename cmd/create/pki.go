// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	certutil "k8s.io/client-go/util/cert"
	pki "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/clusterpki"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

func NewPki(etcd, controlplane bool, pkiPath string) error {
	var (
		err  error
		msg  error
		data clusterpki.ClusterPKI

		cert = certutil.Config{
			CommonName:   "SIGHUP s.r.l. Server",
			Organization: []string{"SIGHUP s.r.l."},
			AltNames:     certutil.AltNames{DNSNames: []string{}, IPs: []net.IP{}},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		certConfig = pki.CertConfig{
			Config:              cert,
			EncryptionAlgorithm: "",
		}
	)

	data.Path = pkiPath
	data.CertConfig = certConfig

	switch {
	default:
		logrus.Debug("creating PKI for etcd and Kubernetes control plane")

		etcd := clusterpki.Etcd{ClusterPKI: data}
		cp := clusterpki.ControlPlanePKI{ClusterPKI: data}

		err = etcd.Create()
		if err != nil {
			msg = fmt.Errorf("got error while creating etcd PKI: %w", err)
		}

		err = cp.Create()
		if err != nil {
			msg = fmt.Errorf("got error while creating control plane PKI: %w", err)
		}

		return msg

	case etcd:
		logrus.Debug("creating PKI for etcd")

		etcd := clusterpki.Etcd{ClusterPKI: data}

		err = etcd.Create()
		if err != nil {
			return fmt.Errorf("creating PKI for etcd failed: %w", err)
		}

	case controlplane:
		logrus.Debug("creating PKI for Kubernetes control plane")

		cp := clusterpki.ControlPlanePKI{ClusterPKI: data}

		err := cp.Create()
		if err != nil {
			return fmt.Errorf("creating PKI for etcd failed: %w", err)
		}
	}

	return errors.ErrUnsupported
}

func NewPKICmd() *cobra.Command {
	var cmdEvent analytics.Event

	pkiCmd := &cobra.Command{
		Use:   "pki",
		Short: "Creates the Public Key Infrastructure files needed for an on-premises cluster.",
		Long: `Creates the Public Key Infrastructure files needed (CA, certificates, keys, etc.) by a Kubernetes cluster and its etcd database.
You can limit the creation of the PKI to just etcd or just Kubernetes using the flags, if not specified the command will create the PKI for both of them.`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			// Get flags
			// maybe we could get this path from the furyctl.yaml file.
			pkiPath := viper.GetString("path")
			etcd := viper.GetBool("etcd")
			controlplane := viper.GetBool("controlplane")

			if err := NewPki(etcd, controlplane, pkiPath); err != nil {
				cmdEvent.AddErrorMessage(err)

				return fmt.Errorf("PKI creation failed with error: %w", err)
			}

			cmdEvent.AddSuccessMessage("PKI files successfully created at:" + pkiPath)
			tracker.Track(cmdEvent)

			return nil
		},
	}

	pkiCmd.Flags().StringP(
		"path",
		"p",
		"pki",
		"path where to save the created PKI files. One subfolder will be created for the control plane files and another one for the etcd files.",
	)

	pkiCmd.Flags().BoolP(
		"etcd",
		"e",
		false,
		"create PKI only for etcd",
	)

	pkiCmd.Flags().BoolP(
		"controlplane",
		"c",
		false,
		"create PKI only for the Kubernetes control plane components",
	)

	return pkiCmd
}
