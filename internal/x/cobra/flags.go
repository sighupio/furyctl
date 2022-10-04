// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
