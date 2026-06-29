// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package template_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/pkg/template"
)

func TestNewTemplateModel(t *testing.T) {
	conf := map[string]any{
		"data": map[string]any{
			"meta": map[string]string{
				"name": "test",
			},
		},
	}

	templateTest := "A nice day at {{.meta.name | substr 0 3}}"

	confYaml, err := yaml.Marshal(conf)
	if err != nil {
		t.Fatal(err)
	}

	path := t.TempDir()

	err = os.Mkdir(path+"/source", os.ModePerm)
	err = os.Mkdir(path+"/target", os.ModePerm)
	err = os.WriteFile(path+"/source/test.md.tpl", []byte(templateTest), os.ModePerm)
	err = os.WriteFile(path+"/configTest.yaml", confYaml, os.ModePerm)

	tm, err := template.NewTemplateModel(
		path+"/source",
		path+"/target",
		path+"/configTest.yaml",
		path,
		"dummy/furyctlconf/path/furyctl.yaml",
		".tpl",
		false,
		false,
	)

	assert.NoError(t, err)
	assert.NotNil(t, tm)
}

func TestGenerate_RelativizesCustomResources(t *testing.T) {
	root := t.TempDir()

	// The furyctl config dir is the anchor for user-supplied relative paths.
	furyctlConfPath := filepath.Join(root, "furyctl.yaml")
	require.NoError(t, os.WriteFile(furyctlConfPath, []byte("apiVersion: test\n"), os.ModePerm))

	// A local kustomize base sitting next to furyctl.yaml.
	localBase := filepath.Join(root, "gapi-calico")
	require.NoError(t, os.MkdirAll(localBase, os.ModePerm))

	// The distribution render target; the kustomization lives under manifests/.
	target := filepath.Join(root, ".furyctl", "cluster", "distribution")
	manifestsDir := filepath.Join(target, distributionManifestsDirForTest)

	remote := "github.com/sighupio/distribution//modules/auth?ref=v1.0.0"

	conf := map[string]any{
		"data": map[string]any{
			"spec": map[string]any{
				"distribution": map[string]any{
					"customResources": []any{
						"./gapi-calico",
						remote,
					},
				},
			},
		},
	}

	confYaml, err := yaml.Marshal(conf)
	require.NoError(t, err)

	source := filepath.Join(root, "source", "manifests")
	require.NoError(t, os.MkdirAll(source, os.ModePerm))

	tpl := "resources:\n" +
		"{{- range .spec.distribution.customResources }}\n" +
		"  - {{ . }}\n" +
		"{{- end }}\n"
	require.NoError(t, os.WriteFile(filepath.Join(source, "kustomization.yaml.tpl"), []byte(tpl), os.ModePerm))

	configPath := filepath.Join(root, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, confYaml, os.ModePerm))

	tm, err := template.NewTemplateModel(
		filepath.Join(root, "source"),
		target,
		configPath,
		root,
		furyctlConfPath,
		".tpl",
		false,
		false,
	)
	require.NoError(t, err)

	require.NoError(t, tm.Generate())

	rendered, err := os.ReadFile(filepath.Join(manifestsDir, "kustomization.yaml"))
	require.NoError(t, err)

	expectedRel, err := filepath.Rel(manifestsDir, localBase)
	require.NoError(t, err)

	// Local base is rewritten to a path relative to the manifests dir...
	assert.Contains(t, string(rendered), "- "+expectedRel)
	assert.NotContains(t, string(rendered), localBase) // ...not an absolute path.
	// Remote resources pass through untouched.
	assert.Contains(t, string(rendered), "- "+remote)
}

// distributionManifestsDirForTest mirrors the unexported constant in the package
// under test; kept here so the external test does not depend on its value leaking.
const distributionManifestsDirForTest = "manifests"
