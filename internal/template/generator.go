package template

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
	"text/template/parse"
)

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

func (g *generator) processTemplate() *template.Template {
	return template.Must(
		template.New(filepath.Base(g.source)).Funcs(g.funcMap.FuncMap).ParseFiles(g.source))
}

func (g *generator) getMissingKeys(tpl *template.Template) []string {
	var missingKeys []string

	fields := getNodeFields(tpl.Tree.Root.Nodes, []string{})

	for _, f := range fields {
		val := g.getContextValueFromPath(f)
		if val == nil {
			missingKeys = append(missingKeys, f)
		}
	}

	return missingKeys
}

func (g *generator) processFile(tpl *template.Template) (bytes.Buffer, error) {
	var generatedContent bytes.Buffer

	if !g.dryRun {
		tpl.Option("missingkey=error")
	}

	err := tpl.Execute(&generatedContent, g.context)

	return generatedContent, err
}

// get all NodeField Values from NodeList recursively
func getNodeFields(nodes []parse.Node, out []string) []string {
	for _, n := range nodes {
		switch n.Type() {
		case parse.NodeList:
			n, ok := n.(*parse.ListNode)
			if ok {
				out = getNodeFields(n.Nodes, out)
			}
		case parse.NodeChain:
		case parse.NodeVariable:
		case parse.NodeField:
			out = append(out, n.String())
		case parse.NodeAction:
			n, ok := n.(*parse.ActionNode)
			if ok {
				for _, cmd := range n.Pipe.Cmds {
					out = getNodeFields(cmd.Args, out)
				}
			}
		case parse.NodeIf:
			n, ok := n.(*parse.IfNode)
			if ok {
				for _, cmd := range n.BranchNode.Pipe.Cmds {
					out = getNodeFields(cmd.Args, out)
				}

				if n.BranchNode.List != nil {
					out = getNodeFields(n.BranchNode.List.Nodes, out)
				}

				if n.BranchNode.ElseList != nil {
					out = getNodeFields(n.BranchNode.ElseList.Nodes, out)
				}
			}
		case parse.NodeTemplate:
			n, ok := n.(*parse.TemplateNode)
			if ok {
				if n.Pipe != nil {
					for _, cmd := range n.Pipe.Cmds {
						out = getNodeFields(cmd.Args, out)
					}
				}
			}
		case parse.NodePipe:
			n, ok := n.(*parse.PipeNode)
			if ok {
				for _, cmd := range n.Cmds {
					out = getNodeFields(cmd.Args, out)
				}
			}
		case parse.NodeRange:
			n, ok := n.(*parse.RangeNode)
			if ok {
				for _, cmd := range n.BranchNode.Pipe.Cmds {
					out = getNodeFields(cmd.Args, out)
				}

				if n.Pipe != nil {
					for _, cmd := range n.Pipe.Cmds {
						out = getNodeFields(cmd.Args, out)
					}
				}

				if n.List != nil {
					out = getNodeFields(n.List.Nodes, out)
				}

				if n.ElseList != nil {
					out = getNodeFields(n.ElseList.Nodes, out)
				}
			}
		}
	}

	return out
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

func (g *generator) processFilename(
	tm *Model,
) (string, error) {
	var realTarget string

	if tm.Config.Templates.ProcessFilename { //try to process filename as template
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
		realTarget = realTarget[:len(realTarget)-len(tm.Suffix)] //cut off extension (.tmpl) from the end
	}

	return realTarget, nil
}

func (g *generator) updateTarget(newTarget string) {
	g.target = newTarget
}
