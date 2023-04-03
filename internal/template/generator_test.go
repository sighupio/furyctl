// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package template_test

import (
	"fmt"
	"os"
	"testing"
	gotemplate "text/template"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/template"
)

type Meta struct {
	Name string `yaml:"name,flow"`
}

func TestTemplateModel_Will_Generate_UserHello(t *testing.T) {
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
		panic(err)
	}

	path, err := os.MkdirTemp("", "test")

	err = os.Mkdir(path+"/source", os.ModePerm)
	err = os.Mkdir(path+"/target", os.ModePerm)
	err = os.WriteFile(path+"/source/test.md.tpl", []byte(templateTest), os.ModePerm)
	err = os.WriteFile(path+"/configTest.yaml", confYaml, os.ModePerm)

	defer os.RemoveAll(path)

	tm, err := template.NewTemplateModel(
		path+"/source",
		path+"/target",
		path+"/configTest.yaml",
		path,
		".tpl",
		false,
		false,
	)

	err = tm.Generate()
	assert.NoError(t, err)

	result, err := os.ReadFile(path + "/target/test.md")
	if err != nil {
		panic(err)
	}

	expectedRes := "A nice day at tes"

	assert.Equal(t, expectedRes, string(result))
}

func TestTemplateModel_Will_Generate_Dynamic_Values_From_Env(t *testing.T) {
	conf := map[string]any{
		"data": map[string]any{
			"meta": Meta{
				Name: "{env://TEST_USER_TYMLATE}",
			},
		},
	}

	templateTest := "A nice day at {{.meta.name | substr 0 3}}"

	confYaml, err := yaml.Marshal(conf)
	if err != nil {
		panic(err)
	}

	path, err := os.MkdirTemp("", "test")

	err = os.Mkdir(path+"/source", os.ModePerm)
	err = os.Mkdir(path+"/target", os.ModePerm)
	err = os.WriteFile(path+"/source/test.md.tpl", []byte(templateTest), os.ModePerm)
	err = os.WriteFile(path+"/configTest.yaml", confYaml, os.ModePerm)

	defer os.RemoveAll(path)

	tm, err := template.NewTemplateModel(
		path+"/source",
		path+"/target",
		path+"/configTest.yaml",
		path,
		".tpl",
		false,
		false,
	)

	os.Setenv("TEST_USER_TYMLATE", "Tymlate")

	defer os.Setenv("TEST_USER_TYMLATE", "")

	err = tm.Generate()
	assert.NoError(t, err)

	result, err := os.ReadFile(path + "/target/test.md")
	if err != nil {
		panic(err)
	}

	expectedRes := "A nice day at Tym"

	assert.Equal(t, expectedRes, string(result))
}

func TestTemplateModel_Will_Generate_Dynamic_Values_From_File(t *testing.T) {
	path, err := os.MkdirTemp("", "test")

	conf := map[string]any{
		"data": map[string]any{
			"meta": Meta{
				Name: fmt.Sprintf("{file://%s/tymlate_test_file.txt}", path),
			},
		},
	}

	templateTest := "A nice day at {{.meta.name}}"

	confYaml, err := yaml.Marshal(conf)
	if err != nil {
		panic(err)
	}

	err = os.Mkdir(path+"/source", os.ModePerm)
	err = os.Mkdir(path+"/target", os.ModePerm)
	err = os.WriteFile(path+"/source/test.md.tpl", []byte(templateTest), os.ModePerm)
	err = os.WriteFile(path+"/configTest.yaml", confYaml, os.ModePerm)

	defer os.RemoveAll(path)

	tm, err := template.NewTemplateModel(
		path+"/source",
		path+"/target",
		path+"/configTest.yaml",
		path,
		".tpl",
		false,
		false,
	)

	exampleStr := "Tymlate! It's a nice day!"

	os.WriteFile(path+"/tymlate_test_file.txt", []byte(exampleStr), os.ModePerm)

	err = tm.Generate()
	assert.NoError(t, err)

	result, err := os.ReadFile(path + "/target/test.md")
	if err != nil {
		panic(err)
	}

	expectedRes := "A nice day at Tymlate! It's a nice day!"

	assert.Equal(t, expectedRes, string(result))
}

func Test_Generator_GetMissingKeys(t *testing.T) {
	path, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}

	funcMap := template.NewFuncMap()
	funcMap.Add("toYaml", template.ToYAML)
	funcMap.Add("fromYaml", template.FromYAML)

	tg := template.NewGenerator(
		path+"/source",
		path+"/source",
		path+"/target",
		map[string]map[any]any{},
		funcMap,
		true,
	)

	tpl := gotemplate.New("test")
	tpl.Parse("{{.meta.name}}")

	missingKeys := tg.GetMissingKeys(tpl)

	assert.Equal(t, 1, len(missingKeys))
	assert.Equal(t, ".meta.name", missingKeys[0])
}
