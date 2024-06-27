// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/legacy"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewLegacyCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	legacyCmd := &cobra.Command{
		Use:   "legacy",
		Short: "Legacy commands for compatibility with older versions of furyctl",
	}

	vendorCmd, err := legacy.NewVendorCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating vendor command: %w", err)
	}

	legacyCmd.AddCommand(vendorCmd)

	return legacyCmd, nil
}
