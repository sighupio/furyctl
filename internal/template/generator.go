// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/iox"
)

var ErrProcessTemplate = errors.New("error processing template")

type generator struct {
	source  string
	target  string
	context map[string]map[any]any
	funcMap FuncMap
	dryRun  bool
}

func NewGenerator(
	source,
	target string,
	context map[string]map[any]any,
	funcMap FuncMap,
	dryRun bool,
) *generator {
	return &generator{
		source:  source,
		target:  target,
		context: context,
		funcMap: funcMap,
		dryRun:  dryRun,
	}
}

func (g *generator) ProcessTemplate() (*template.Template, error) {
	const helpersPath = "source/_helpers.tpl"

	_, err := os.Stat(helpersPath)

	if err == nil {
		return template.New(filepath.Base(g.source)).Funcs(g.funcMap.FuncMap).ParseFiles(g.source, helpersPath)
	}

	if errors.Is(err, os.ErrNotExist) {
		logrus.Warnf("template helpers file '%s' not found\n", helpersPath)

		return template.New(filepath.Base(g.source)).Funcs(g.funcMap.FuncMap).ParseFiles(g.source)
	}

	return nil, fmt.Errorf("%w using helper '%s': %v", ErrProcessTemplate, helpersPath, err)
}

func (g *generator) GetMissingKeys(tpl *template.Template) []string {
	var missingKeys []string

	node := NewNode()
	node.FromNodeList(tpl.Tree.Root.Nodes)

	for _, f := range node.Fields {
		val := g.getContextValueFromPath(f)
		if val == nil {
			missingKeys = append(missingKeys, f)
		}
	}

	return missingKeys
}

func (g *generator) ProcessFile(tpl *template.Template) (bytes.Buffer, error) {
	var generatedContent bytes.Buffer

	if !g.dryRun {
		tpl.Option("missingkey=error")
	}

	err := tpl.Execute(&generatedContent, g.context)

	return generatedContent, err
}

func (g *generator) ProcessFilename(
	tm *Model,
) (string, error) {
	var realTarget string

	if tm.Config.Templates.ProcessFilename { // try to process filename as template
		tpl := template.Must(
			template.New("currentTarget").Funcs(g.funcMap.FuncMap).Parse(g.target))

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
		realTarget = realTarget[:len(realTarget)-len(tm.Suffix)] // cut off extension (.tmpl) from the end
	}

	return realTarget, nil
}

func (g *generator) UpdateTarget(newTarget string) {
	g.target = newTarget
}

func (g *generator) WriteMissingKeysToFile(
	missingKeys []string,
	tmplPath,
	outputPath string,
) error {
	if len(missingKeys) == 0 {
		return nil
	}

	logrus.Warnf(
		"missing keys in template %s. Writing to %s/tmpl-debug.log\n",
		tmplPath,
		outputPath,
	)

	debugFilePath := filepath.Join(outputPath, "tmpl-debug.log")

	outLog := fmt.Sprintf("[%s]\n%s\n", tmplPath, strings.Join(missingKeys, "\n"))

	return iox.AppendBufferToFile(*bytes.NewBufferString(outLog), debugFilePath)
}

func (g *generator) getContextValueFromPath(path string) any {
	paths := strings.Split(path[1:], ".")

	if len(paths) == 0 {
		return nil
	}

	ret := g.context[paths[0]]

	for _, key := range paths[1:] {
		mapAtKey, ok := ret[key]
		if !ok {
			return nil
		}

		ret, ok = mapAtKey.(map[any]any)
		if !ok {
			return mapAtKey
		}
	}

	return ret
}
