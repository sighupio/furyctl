// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"
)

const (
	GithubSSHRepoPrefix   = "git@github.com:sighupio"
	GithubHTTPSRepoPrefix = "https://github.com/sighupio"
)

func RepoPrefixByProtocol(protocol Protocol) (string, error) {
	switch protocol {
	case ProtocolSSH:
		return GithubSSHRepoPrefix, nil

	case ProtocolHTTPS:
		return GithubHTTPSRepoPrefix, nil

	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedGitProtocol, protocol)
	}
}

func StripPrefix(repo string) string {
	strippedRepo := strings.TrimPrefix(repo, GithubSSHRepoPrefix+"/")
	strippedRepo = strings.TrimPrefix(strippedRepo, GithubHTTPSRepoPrefix+"/")

	return strippedRepo
}

func StripSuffix(repo string) string {
	return strings.TrimSuffix(repo, ".git")
}

func CleanupRepoURL(repo string) string {
	strippedRepo := StripPrefix(repo)
	strippedRepo = StripSuffix(strippedRepo)

	return strippedRepo
}
