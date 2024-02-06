// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import "strings"

const (
	GithubSSHRepoPrefix   = "git@github.com:sighupio"
	GithubHTTPSRepoPrefix = "https://github.com/sighupio"
)

func RepoPrefixByProtocol(protocol Protocol) string {
	if protocol == ProtocolSSH {
		return GithubSSHRepoPrefix
	}

	return GithubHTTPSRepoPrefix
}

func StripPrefix(repo string) string {
	strippedRepo := strings.TrimPrefix(repo, GithubSSHRepoPrefix+"/")
	strippedRepo = strings.TrimPrefix(strippedRepo, GithubHTTPSRepoPrefix+"/")

	return strippedRepo
}
