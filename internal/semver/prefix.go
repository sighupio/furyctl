// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import "strings"

const prefix = "v"

func EnsurePrefix(version string) string {
	if !strings.HasPrefix(version, prefix) {
		return prefix + version
	}

	return version
}

func EnsureNoPrefix(version string) string {
	if strings.HasPrefix(version, prefix) {
		return strings.TrimPrefix(version, prefix)
	}

	return version
}
