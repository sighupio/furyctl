package template

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v2"
)

func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

func funcMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	f["toYaml"] = toYAML
	f["fromYaml"] = fromYAML
	return f
}

type generator struct {
	source  string
	target  string
	context map[string]map[interface{}]interface{}
}

func NewGenerator(source, target string, context map[string]map[interface{}]interface{}) *generator {
	return &generator{
		source:  source,
		target:  target,
		context: context,
	}
}

func (g *generator) processFile() (bytes.Buffer, error) {
	var generatedContent bytes.Buffer

	tpl := template.Must(
		template.New(filepath.Base(g.source)).Funcs(funcMap()).ParseFiles(g.source))

	if err := tpl.Execute(&generatedContent, g.context); err != nil {
		return generatedContent, err
	}

	return generatedContent, nil
}

func (g *generator) processFilename(
	tm *Model,
) (string, error) {
	var realTarget string

	if tm.Config.Templates.ProcessFilename { //try to process filename as template
		tpl := template.Must(
			template.New("currentTarget").Funcs(funcMap()).Parse(g.target))

		destination := bytes.NewBufferString("")

		if err := tpl.Execute(destination, g.context); err != nil {
			return "", err
		}
		realTarget = destination.String()
	} else {
		realTarget = g.target
	}

	suf := tm.Suffix
	if strings.HasSuffix(realTarget, suf) {
		realTarget = realTarget[:len(realTarget)-len(tm.Suffix)] //cut off extension (.tmpl) from the end
	}

	return realTarget, nil
}

func (g *generator) updateTarget(newTarget string) {
	g.target = newTarget
}
