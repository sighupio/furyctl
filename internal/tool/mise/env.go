// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mise

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/samber/lo"
)

// parseEnvJSON converts the output of `mise env --json` ({"KEY":"VALUE", ...}) into a sorted slice
// of "KEY=VALUE" entries suitable for exec.Cmd.Env.
func parseEnvJSON(s string) ([]string, error) {
	m := map[string]string{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("error parsing mise env json: %w", err)
	}

	out := lo.MapToSlice(m, func(k, v string) string {
		return k + "=" + v
	})

	slices.Sort(out)

	return out, nil
}
