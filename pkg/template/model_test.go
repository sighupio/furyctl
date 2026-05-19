// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package templatex_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	templatex "github.com/sighupio/furyctl/pkg/template"
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

	tm, err := templatex.NewTemplateModel(
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
