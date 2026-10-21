// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

// pluginKinds tells whether the rendered configuration has kustomize plugins and helm releases. It reads the
// configuration whichever YAML decoder gave it.
func pluginKinds(rendered map[string]any) (bool, bool) {
	specPlugins := asAnyMap(asAnyMap(rendered["spec"])["plugins"])
	releases, ok := releasesOf(specPlugins).([]any)

	return hasKustomizeEntries(specPlugins), ok && len(releases) > 0
}

// hasHelmReleases tells whether the spec.plugins map of the template data has at least one helm release. The
// template data is decoded with yaml.v2, so its maps are map[any]any.
func hasHelmReleases(specPlugins map[any]any) bool {
	helm, ok := specPlugins["helm"].(map[any]any)
	if !ok {
		return false
	}

	releases, ok := helm["releases"].([]any)

	return ok && len(releases) > 0
}

// hasKustomizeEntries tells whether a spec.plugins map has at least one kustomize plugin.
func hasKustomizeEntries(specPlugins map[any]any) bool {
	kustomize, ok := specPlugins["kustomize"].([]any)

	return ok && len(kustomize) > 0
}

// releasesOf is the spec.plugins.helm.releases list of a spec.plugins map.
func releasesOf(specPlugins map[any]any) any {
	return asAnyMap(specPlugins["helm"])["releases"]
}

// pluginNames lists the name of each entry of a spec.plugins list.
func pluginNames(list any) []string {
	entries, ok := list.([]any)
	if !ok {
		return []string{}
	}

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		if name, ok := asAnyMap(entry)["name"].(string); ok {
			names = append(names, name)
		}
	}

	return names
}

// asAnyMap reads a decoded YAML map whichever the type of its keys is.
func asAnyMap(v any) map[any]any {
	switch m := v.(type) {
	case map[any]any:
		return m

	case map[string]any:
		out := make(map[any]any, len(m))
		for k, val := range m {
			out[k] = val
		}

		return out

	default:
		return nil
	}
}
