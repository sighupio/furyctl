package cobrax

import (
	"strconv"

	"github.com/spf13/cobra"
)

func Flag[T bool | int | string](cmd *cobra.Command, name string) any {
	var f T

	if cmd == nil {
		return f
	}

	if cmd.Flag(name) == nil {
		return f
	}

	v := cmd.Flag(name).Value.String()

	if v == "true" {
		return true
	}

	if v == "false" {
		return false
	}

	if vv, err := strconv.Atoi(v); err == nil {
		return vv
	}

	return v
}
