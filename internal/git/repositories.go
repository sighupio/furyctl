package git

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
