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

	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Generator struct {
	rootSrc string
	source  string
	target  string
	context map[string]map[any]any
	funcMap FuncMap
	dryRun  bool
}

func NewGenerator(
	rootSrc,
	source,
	target string,
	context map[string]map[any]any,
	funcMap FuncMap,
	dryRun bool,
) *Generator {
	return &Generator{
		rootSrc: rootSrc,
		source:  source,
		target:  target,
		context: context,
		funcMap: funcMap,
		dryRun:  dryRun,
	}
}

func (g *Generator) ProcessTemplate() (*template.Template, error) {
	helpersPath := filepath.Join(g.rootSrc, "_helpers.tpl")

	_, err := os.Stat(helpersPath)
	if err == nil {
		tpl, err := template.New(filepath.Base(g.source)).Funcs(g.funcMap.FuncMap).ParseFiles(g.source, helpersPath)
		if err != nil {
			return nil, fmt.Errorf("error processing template: %w", err)
		}

		return tpl, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		tpl, err := template.New(filepath.Base(g.source)).Funcs(g.funcMap.FuncMap).ParseFiles(g.source)
		if err != nil {
			return nil, fmt.Errorf("error processing template: %w", err)
		}

		return tpl, nil
	}

	return nil, fmt.Errorf("error processing template using helper '%s': %w", helpersPath, err)
}

func (g *Generator) GetMissingKeys(tpl *template.Template) []string {
	var missingKeys []string

	if tpl == nil || tpl.Tree == nil || tpl.Tree.Root == nil {
		return missingKeys
	}

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

func (g *Generator) ProcessFile(tpl *template.Template) (bytes.Buffer, error) {
	var generatedContent bytes.Buffer

	if !g.dryRun {
		tpl.Option("missingkey=error")
	}

	err := tpl.Execute(&generatedContent, g.context)
	if err != nil {
		return generatedContent, fmt.Errorf("error processing template: %w", err)
	}

	return generatedContent, nil
}

func (g *Generator) ProcessFilename(
	tm *Model,
) (string, error) {
	var realTarget string

	if tm.Config.Templates.ProcessFilename { // Try to process filename as template.
		tpl := template.Must(
			template.New("currentTarget").Funcs(g.funcMap.FuncMap).Parse(g.target))

		destination := bytes.NewBufferString("")

		if err := tpl.Execute(destination, g.context); err != nil {
			return "", fmt.Errorf("error processing filename: %w", err)
		}

		realTarget = destination.String()
	} else {
		realTarget = g.target
	}

	suf := tm.Suffix
	if strings.HasSuffix(realTarget, suf) {
		realTarget = realTarget[:len(realTarget)-len(tm.Suffix)] // Cut off extension (.tmpl) from the end.
	}

	return realTarget, nil
}

func (g *Generator) UpdateTarget(newTarget string) {
	g.target = newTarget
}

func (*Generator) WriteMissingKeysToFile(
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

	err := iox.AppendToFile(outLog, debugFilePath)
	if err != nil {
		return fmt.Errorf("error writing missing keys to log file: %w", err)
	}

	return nil
}

func (g *Generator) getContextValueFromPath(path string) any {
	paths := strings.Split(path[1:], ".")

	if len(paths) == 0 {
		return nil
	}

	ret := g.context[paths[0]]

	for i, key := range paths[1:] {
		mapAtKey, ok := ret[key]
		if !ok {
			return nil
		}

		ret, ok = mapAtKey.(map[any]any)
		if !ok {
			if i == len(paths)-2 {
				return mapAtKey
			}

			return nil
		}
	}

	if len(ret) == 0 {
		return nil
	}

	return ret
}
