// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"reflect"
	"strings"
	"text/template/parse"

	"github.com/sighupio/furyctl/internal/x/slices"
)

type Node struct {
	Fields []string
}

func NewNode() *Node {
	return &Node{
		Fields: []string{},
	}
}

func (f *Node) Set(s []string) {
	f.Fields = s
}

func (f *Node) FromNodeList(nodes []parse.Node) []string {
	for _, n := range nodes {
		setter, ok := mapToAliasInterface(n).(FieldsSetter)
		if ok {
			setter.Set(f)
		}
	}

	return slices.Uniq(f.Fields)
}

func mapToAliasInterface(n parse.Node) interface{} {
	// MapParseNodeToAlias is a map of parse.Node to its alias.
	mapParseNodeToAlias := map[parse.NodeType]interface{}{
		parse.NodeList:     &ListNode{},
		parse.NodeRange:    &RangeNode{},
		parse.NodePipe:     &PipeNode{},
		parse.NodeTemplate: &TplNode{},
		parse.NodeIf:       &IfNode{},
		parse.NodeAction:   &ActionNode{},
		parse.NodeField:    &FieldNode{},
		parse.NodeVariable: &VariableNode{},
	}

	t := mapParseNodeToAlias[n.Type()]

	if t == nil {
		return nil
	}

	return reflect.ValueOf(n).Convert(reflect.TypeOf(t)).Interface()
}

type FieldsSetter interface {
	Set(n *Node)
}

type ListNode parse.ListNode

func (l *ListNode) Set(n *Node) {
	n.Set(n.FromNodeList(l.Nodes))
}

type RangeNode parse.RangeNode

func (r *RangeNode) Set(n *Node) {
	if r.Pipe != nil {
		for _, cmd := range r.Pipe.Cmds {
			n.Set(n.FromNodeList(cmd.Args))
		}
	}

	if r.List != nil {
		n.Set(n.FromNodeList(r.List.Nodes))
	}

	if r.ElseList != nil {
		n.Set(n.FromNodeList(r.ElseList.Nodes))
	}
}

type PipeNode parse.PipeNode

func (p *PipeNode) Set(n *Node) {
	for _, cmd := range p.Cmds {
		n.Set(n.FromNodeList(cmd.Args))
	}
}

type TplNode parse.TemplateNode

func (t *TplNode) Set(n *Node) {
	if t.Pipe != nil {
		for _, cmd := range t.Pipe.Cmds {
			n.Set(n.FromNodeList(cmd.Args))
		}
	}
}

type IfNode parse.IfNode

func (i *IfNode) Set(n *Node) {
	for _, cmd := range i.BranchNode.Pipe.Cmds {
		n.Set(n.FromNodeList(cmd.Args))
	}

	if i.BranchNode.List != nil {
		n.Set(n.FromNodeList(i.BranchNode.List.Nodes))
	}

	if i.BranchNode.ElseList != nil {
		n.Set(n.FromNodeList(i.BranchNode.ElseList.Nodes))
	}
}

type ActionNode parse.ActionNode

func (a *ActionNode) Set(n *Node) {
	for _, cmd := range a.Pipe.Cmds {
		n.Set(n.FromNodeList(cmd.Args))
	}
}

type FieldNode parse.FieldNode

func (f *FieldNode) Set(n *Node) {
	n.Set(append(n.Fields, stringsToPath(f.Ident)))
}

type VariableNode parse.VariableNode

// Set Skips VariableNodes because they are not fields.
func (*VariableNode) Set(_ *Node) {
	// Do nothing.
}

func stringsToPath(s []string) string {
	if len(s) == 0 {
		return ""
	}

	return "." + strings.Join(s, ".")
}
