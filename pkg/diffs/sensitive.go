// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diffs

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

const (
	// HiddenValue is printed in place of a value that must not be shown.
	HiddenValue = "(hidden)"

	// A segment * of a path pattern matches any one segment, a key or a list position.
	anySegment = "*"
)

// sensitiveWords are the words that make a key sensitive. The configuration schema
// carries no marker for secret fields, so the names of the keys are the main source.
func sensitiveWords() []string {
	return []string{"pass", "secret", "token", "key", "credential", "authorization"}
}

// secretPaths are the fields that hold credentials although no word of their key says
// so: free text, file contents, webhook URLs and the values of free-form pairs. A path
// at or below one of them is sensitive.
func secretPaths() []string {
	return []string{
		"spec.plugins.helm.releases.*.set.*.value",
		"spec.distribution.modules.auth.dex.connectors.*.config.bindPW",
		"spec.distribution.modules.monitoring.alertmanager.slackWebhookUrl",
		"spec.distribution.modules.monitoring.alertmanager.deadManSwitchWebhookUrl",
		"spec.distribution.modules.monitoring.*.remoteWrite.*.headers",
		"spec.distribution.modules.logging.customOutputs",
		"spec.distribution.customPatches.patches.*.patch",
		"spec.distribution.customPatches.patchesStrategicMerge",
		"spec.kubernetes.advanced.encryption.configuration",
		"spec.kubernetes.advanced.cloud.config",
		"spec.kubernetes.loadBalancers.additionalConfig",
		"spec.infrastructure.loadBalancers.haproxy.configuration",
		"spec.infrastructure.ipxeServer.preInstallCommands",
		"spec.infrastructure.ipxeServer.postInstallCommands",
		"spec.infrastructure.nodes.*.storage.files.*.contents.inline",
		"spec.infrastructure.nodes.*.storage.files.*.contents.source",
		"spec.infrastructure.nodes.*.storage.files.*.contents.http_headers.*.value",
		"spec.infrastructure.nodes.*.storage.files.*.append.*.inline",
		"spec.infrastructure.nodes.*.storage.files.*.append.*.source",
		"spec.infrastructure.nodes.*.storage.files.*.append.*.http_headers.*.value",
		"spec.infrastructure.nodes.*.systemd.units.*.contents",
		"spec.infrastructure.nodes.*.systemd.units.*.dropins.*.contents",
	}
}

// plainPaths are the fields whose key holds a sensitive word but whose value is not a
// secret: the path of a key file, the prefix of a state object, a security switch.
func plainPaths() []string {
	return []string{
		"spec.kubernetes.ssh.keyPath",
		"spec.infrastructure.ssh.privateKeyPath",
		"spec.infrastructure.ssh.publicKeyPath",
		"spec.kubernetes.nodePoolsCommon.metadataHttpTokens",
		"spec.toolsConfiguration.*.state.s3.keyPrefix",
	}
}

// plainEndings are the endings of the paths of plain fields found at any depth: the
// key of a toleration or of a taint names a node label.
func plainEndings() []string {
	return []string{"tolerations.*.key", "taints.*.key"}
}

// isSensitiveKey reports whether a key contains one of the sensitive words, ignoring case.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)

	for _, word := range sensitiveWords() {
		if strings.Contains(lower, word) {
			return true
		}
	}

	return false
}

// isPlainField reports whether the last key of the path is not a secret although its
// name holds a sensitive word.
func isPlainField(path []string) bool {
	for _, pattern := range plainPaths() {
		if matchesPattern(path, pattern) && len(path) == len(strings.Split(pattern, ".")) {
			return true
		}
	}

	for _, pattern := range plainEndings() {
		if start := len(path) - len(strings.Split(pattern, ".")); start >= 0 && matchesPattern(path[start:], pattern) {
			return true
		}
	}

	return false
}

// matchesPattern reports whether the path starts with the segments of the pattern.
func matchesPattern(path []string, pattern string) bool {
	segments := strings.Split(pattern, ".")

	if len(path) < len(segments) {
		return false
	}

	for i, segment := range segments {
		if segment != anySegment && segment != path[i] {
			return false
		}
	}

	return true
}

// IsListPosition reports whether a path segment is a position in a list: digits
// only. A position is not a key, so it is never sensitive, and two positions are
// ordered as numbers.
func IsListPosition(segment string) bool {
	if segment == "" {
		return false
	}

	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// IsSensitivePath reports whether a change at the path holds a secret: one of its key
// segments is sensitive, or the path is at or below one of the secret paths.
// Everything below a sensitive key is sensitive too.
func IsSensitivePath(path []string) bool {
	for i, segment := range path {
		if !IsListPosition(segment) && isSensitiveKey(segment) && !isPlainField(path[:i+1]) {
			return true
		}
	}

	for _, pattern := range secretPaths() {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	return false
}

// MaskSensitive gives a deep copy of the value found at the path in which every
// sensitive value, at any depth, is replaced by HiddenValue, and the user and password
// of a URL are hidden. The value given is not changed. Objects come back as
// map[string]any and lists as []any.
func MaskSensitive(path []string, value any) any {
	if value == nil {
		return nil
	}

	if IsSensitivePath(path) {
		return HiddenValue
	}

	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.Map:
		masked := make(map[string]any, v.Len())

		for _, key := range v.MapKeys() {
			name := fmt.Sprint(key.Interface())

			masked[name] = MaskSensitive(childPath(path, name), v.MapIndex(key).Interface())
		}

		return masked

	case reflect.Slice, reflect.Array:
		masked := make([]any, v.Len())

		for i := range v.Len() {
			masked[i] = MaskSensitive(childPath(path, strconv.Itoa(i)), v.Index(i).Interface())
		}

		return masked

	case reflect.String:
		return maskUserinfo(v.String())

	default:
		return value
	}
}

// maskUserinfo hides the user and password of a text that is a URL, such as
// https://user:password@host/path. Any other text stays as it is.
func maskUserinfo(text string) string {
	parsed, err := url.Parse(text)
	if err != nil || parsed.User == nil || parsed.Host == "" {
		return text
	}

	start := strings.Index(text, "://")
	if start < 0 {
		return text
	}

	start += len("://")

	end := len(text)
	if i := strings.IndexAny(text[start:], "/?#"); i >= 0 {
		end = start + i
	}

	at := strings.LastIndex(text[start:end], "@")
	if at < 0 {
		return text
	}

	return text[:start] + HiddenValue + text[start+at:]
}
