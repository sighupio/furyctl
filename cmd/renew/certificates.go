// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package renew

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
)

func NewCertificatesCmd() *cobra.Command {
	var cmdEvent analytics.Event

	certificatesCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "certificates",
		Short: "Renew the PKI certificates of a cluster",
		Long: "Renew the certificates of the cluster PKI. The cluster components use these certificates " +
			"to authenticate.\n" +
			"This command does not renew other certificates, for example the Ingress certificates or the " +
			"certificates that cert-manager controls.",
		Example: `  furyctl renew certificates                            Renew the certificates of the cluster PKI
  furyctl renew certificates --config mycluster.yaml    Renew with a custom configuration file
`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = preRun(cmd)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			renewer, err := newRenewer(cmdEvent, tracker)
			if err != nil {
				return err
			}

			if err := renewer.RenewCertificates(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while renewing certificates: %w", err)
			}

			logrus.Info("Certificates successfully renewed")

			cmdEvent.AddSuccessMessage("certificates successfully renewed")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	registerFlags(certificatesCmd)

	return certificatesCmd
}
