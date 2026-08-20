// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package renew

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
)

var ErrUnknownKubeconfig = errors.New("unknown kubeconfig")

func NewKubeconfigsCmd() *cobra.Command {
	var cmdEvent analytics.Event

	kubeconfigsCmd := &cobra.Command{
		Args:  cobra.ArbitraryArgs,
		Use:   "kubeconfigs [NAME...]",
		Short: "Renew the kubeconfig files of a cluster",
		Long: "Renew the kubeconfig file of the admin and the kubeconfig files of the users in " +
			"`spec.kubernetes.advanced.users.names`.\n" +
			"With no name, furyctl renews all of them. A space or a comma separates the names.\n" +
			"furyctl writes the renewed kubeconfig files in the current directory.",
		Example: `  furyctl renew kubeconfigs                            Renew the kubeconfig files of the admin and of all the users
  furyctl renew kubeconfigs admin                      Renew only the kubeconfig file of the admin
  furyctl renew kubeconfigs alice bob                  Renew the kubeconfig files of two users
  furyctl renew kubeconfigs --config mycluster.yaml    Renew with a custom configuration file
`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = preRun(cmd)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			renewer, err := newRenewer(cmdEvent, tracker)
			if err != nil {
				return err
			}

			users, err := selectKubeconfigs(args, renewer.Users())
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			if err := renewer.RenewKubeconfigs(users); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while renewing the kubeconfig files: %w", err)
			}

			logrus.Infof("Kubeconfig files successfully renewed: %s", strings.Join(users, ", "))

			cmdEvent.AddSuccessMessage("kubeconfig files successfully renewed")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	registerFlags(kubeconfigsCmd)

	return kubeconfigsCmd
}

// selectKubeconfigs reads the comma separated list of names given on the command line and makes
// sure the configuration file defines all of them. No names selects all of the available ones.
func selectKubeconfigs(args, available []string) ([]string, error) {
	// The kind does not support the renewal: let the renewer report it.
	if len(available) == 0 {
		return nil, nil
	}

	// Joining on the comma accepts both `admin alice` and the `admin,alice` form, and drops the
	// empty fields a trailing comma or `admin, alice` leaves behind.
	names := strings.FieldsFunc(strings.Join(args, ","), func(r rune) bool { return r == ',' })

	if len(names) == 0 {
		return available, nil
	}

	for _, name := range names {
		if !slices.Contains(available, name) {
			return nil, fmt.Errorf(
				"%w %q: the configuration file defines %s",
				ErrUnknownKubeconfig,
				name,
				strings.Join(available, ", "),
			)
		}
	}

	return names, nil
}
