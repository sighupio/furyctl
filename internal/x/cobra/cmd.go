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
