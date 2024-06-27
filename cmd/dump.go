// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd/dump"
	"github.com/sighupio/furyctl/internal/analytics"
)

func NewDumpCommand(tracker *analytics.Tracker) (*cobra.Command, error) {
	dumpCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump manifests templates and other useful KFD objects",
	}

	templateCmd, err := dump.NewTemplateCmd(tracker)
	if err != nil {
		return nil, fmt.Errorf("error while creating template command: %w", err)
	}

	dumpCmd.AddCommand(templateCmd)

	return dumpCmd, nil
}
