// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cobrax

import (
	"strconv"

	"github.com/spf13/cobra"
)

func Flag[T bool | int | string](cmd *cobra.Command, name string) T {
	var (
		f T
		g any
	)

	if cmd == nil {
		return f
	}

	if cmd.Flag(name) == nil {
		return f
	}

	sv := cmd.Flag(name).Value.String()

	if sv == "true" {
		g = true
	} else if sv == "false" {
		g = false
	} else if iv, err := strconv.Atoi(sv); err == nil {
		g = iv
	} else {
		g = sv
	}

	v, ok := g.(T)
	if !ok {
		return f
	}

	return v
}
