package semver

import "strings"

func EnsurePrefix(version, prefix string) string {
	if !strings.HasPrefix(version, prefix) {
		return prefix + version
	}
	return version
}

func EnsureNoPrefix(version, prefix string) string {
	if strings.HasPrefix(version, prefix) {
		return strings.TrimPrefix(version, prefix)
	}
	return version
}
