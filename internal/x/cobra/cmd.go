// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cobrax

import (
	"fmt"

	"github.com/spf13/cobra"
)

// GetFullname returns the hierarchy of the command and its parents. For example: "<command> <subcommand>...".
func GetFullname(c *cobra.Command) string {
	if c.Parent() == nil || c.Parent().Name() == "furyctl" {
		return c.Name()
	}

	return fmt.Sprintf("%s %s", GetFullname(c.Parent()), c.Name())
}
