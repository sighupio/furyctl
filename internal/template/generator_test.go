package template_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/template"
)

type Meta struct {
	Name map[string]any `yaml:"name,flow"`
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

	tm, err := template.NewTemplateModel(path+"/source", path+"/target", path+"/configTest.yaml", ".tpl", false)

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
				Name: map[string]any{"env://TEST_USER_TYMLATE": ""},
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

	tm, err := template.NewTemplateModel(path+"/source", path+"/target", path+"/configTest.yaml", ".tpl", false)

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
				Name: map[string]any{"file://" + path + "/tymlate_test_file.txt": ""},
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

	tm, err := template.NewTemplateModel(path+"/source", path+"/target", path+"/configTest.yaml", ".tpl", false)

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
