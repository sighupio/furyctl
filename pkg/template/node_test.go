// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package templatex_test

import (
	"reflect"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/stretchr/testify/assert"

	templatex "github.com/sighupio/furyctl/pkg/template"
)

func TestNewNode(t *testing.T) {
	node := templatex.NewNode()

	assert.Equal(t, []string{}, node.Fields)
}

func TestNode_FromNodeList(t *testing.T) {
	node := templatex.NewNode()
	tmpl := template.Must(template.New("test").Parse("{{.field1}}"))
	node.FromNodeList(tmpl.Root.Nodes)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestActionNode_Set(t *testing.T) {
	node := templatex.NewNode()
	actionNode := &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pos:      1,
		Line:     1,
		Pipe: &parse.PipeNode{
			NodeType: parse.NodePipe,
			Pos:      1,
			Line:     1,
			IsAssign: false,
			Decl:     nil,
			Cmds: []*parse.CommandNode{
				{
					NodeType: parse.NodeCommand,
					Pos:      1,
					Args: []parse.Node{
						&parse.FieldNode{
							NodeType: parse.NodeField,
							Pos:      1,
							Ident:    []string{"field1"},
						},
					},
				},
			},
		},
	}

	tmplActionNode := reflect.ValueOf(actionNode).Convert(reflect.TypeFor[*templatex.ActionNode]()).Interface()

	assert.NotNil(t, tmplActionNode)

	actionNodeSetter, ok := tmplActionNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	actionNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestFieldNode_Set(t *testing.T) {
	node := templatex.NewNode()
	fieldNode := &parse.FieldNode{
		NodeType: parse.NodeField,
		Pos:      1,
		Ident:    []string{"field1"},
	}

	tmplFieldNode := reflect.ValueOf(fieldNode).Convert(reflect.TypeFor[*templatex.FieldNode]()).Interface()

	assert.NotNil(t, tmplFieldNode)

	fieldNodeSetter, ok := tmplFieldNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	fieldNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestIfNode_Set(t *testing.T) {
	node := templatex.NewNode()
	ifNode := &parse.IfNode{
		BranchNode: parse.BranchNode{
			NodeType: parse.NodeIf,
			Pos:      1,
			Line:     1,
			Pipe: &parse.PipeNode{
				NodeType: parse.NodePipe,
				Pos:      1,
				Line:     1,
				IsAssign: false,
				Decl:     nil,
				Cmds: []*parse.CommandNode{
					{
						NodeType: parse.NodeCommand,
						Pos:      1,
						Args: []parse.Node{
							&parse.FieldNode{
								NodeType: parse.NodeField,
								Pos:      1,
								Ident:    []string{"field1"},
							},
						},
					},
				},
			},
			List: &parse.ListNode{
				NodeType: parse.NodeList,
				Pos:      1,
				Nodes: []parse.Node{
					&parse.FieldNode{
						NodeType: parse.NodeField,
						Pos:      1,
						Ident:    []string{"field2"},
					},
				},
			},
			ElseList: &parse.ListNode{
				NodeType: parse.NodeList,
				Pos:      1,
				Nodes: []parse.Node{
					&parse.FieldNode{
						NodeType: parse.NodeField,
						Pos:      1,
						Ident:    []string{"field3"},
					},
				},
			},
		},
	}

	tmplIfNode := reflect.ValueOf(ifNode).Convert(reflect.TypeFor[*templatex.IfNode]()).Interface()

	assert.NotNil(t, tmplIfNode)

	ifNodeSetter, ok := tmplIfNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	ifNodeSetter.Set(node)

	assert.Equal(t, []string{".field1", ".field2", ".field3"}, node.Fields)
}

func TestListNode_Set(t *testing.T) {
	node := templatex.NewNode()
	listNode := &parse.ListNode{
		NodeType: parse.NodeList,
		Pos:      1,
		Nodes: []parse.Node{
			&parse.FieldNode{
				NodeType: parse.NodeField,
				Pos:      1,
				Ident:    []string{"field1"},
			},
			&parse.FieldNode{
				NodeType: parse.NodeField,
				Pos:      1,
				Ident:    []string{"field2"},
			},
		},
	}

	tmplListNode := reflect.ValueOf(listNode).Convert(reflect.TypeFor[*templatex.ListNode]()).Interface()

	assert.NotNil(t, tmplListNode)

	listNodeSetter, ok := tmplListNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	listNodeSetter.Set(node)

	assert.Equal(t, []string{".field1", ".field2"}, node.Fields)
}

func TestNode_Set(t *testing.T) {
	node := templatex.NewNode()

	node.Set([]string{".field1", ".field2"})

	assert.Equal(t, []string{".field1", ".field2"}, node.Fields)
}

func TestVariableNode_Set(t *testing.T) {
	node := templatex.NewNode()
	variableNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Pos:      1,
		Ident:    []string{"variable1"},
	}

	tmplVariableNode := reflect.ValueOf(variableNode).Convert(reflect.TypeFor[*templatex.VariableNode]()).Interface()

	assert.NotNil(t, tmplVariableNode)

	variableNodeSetter, ok := tmplVariableNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	variableNodeSetter.Set(node)

	assert.Equal(t, []string{}, node.Fields)
}

func TestRangeNode_Set(t *testing.T) {
	node := templatex.NewNode()
	rangeNode := &parse.RangeNode{
		BranchNode: parse.BranchNode{
			NodeType: parse.NodeIf,
			Pos:      1,
			Line:     1,
			Pipe: &parse.PipeNode{
				NodeType: parse.NodePipe,
				Pos:      1,
				Line:     1,
				IsAssign: false,
				Decl:     nil,
				Cmds: []*parse.CommandNode{
					{
						NodeType: parse.NodeCommand,
						Pos:      1,
						Args: []parse.Node{
							&parse.FieldNode{
								NodeType: parse.NodeField,
								Pos:      1,
								Ident:    []string{"field1"},
							},
						},
					},
				},
			},
			List: &parse.ListNode{
				NodeType: parse.NodeList,
				Pos:      1,
				Nodes: []parse.Node{
					&parse.FieldNode{
						NodeType: parse.NodeField,
						Pos:      1,
						Ident:    []string{"field2"},
					},
				},
			},
			ElseList: &parse.ListNode{
				NodeType: parse.NodeList,
				Pos:      1,
				Nodes: []parse.Node{
					&parse.FieldNode{
						NodeType: parse.NodeField,
						Pos:      1,
						Ident:    []string{"field3"},
					},
				},
			},
		},
	}

	tmplRangeNode := reflect.ValueOf(rangeNode).Convert(reflect.TypeFor[*templatex.RangeNode]()).Interface()

	assert.NotNil(t, tmplRangeNode)

	rangeNodeSetter, ok := tmplRangeNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	rangeNodeSetter.Set(node)

	assert.Equal(t, []string{".field1", ".field2", ".field3"}, node.Fields)
}

func TestPipeNode_Set(t *testing.T) {
	node := templatex.NewNode()
	pipeNode := &parse.PipeNode{
		NodeType: parse.NodePipe,
		Pos:      1,
		Line:     1,
		IsAssign: false,
		Decl:     nil,
		Cmds: []*parse.CommandNode{
			{
				NodeType: parse.NodeCommand,
				Pos:      1,
				Args: []parse.Node{
					&parse.FieldNode{
						NodeType: parse.NodeField,
						Pos:      1,
						Ident:    []string{"field1"},
					},
				},
			},
		},
	}

	tmplPipeNode := reflect.ValueOf(pipeNode).Convert(reflect.TypeFor[*templatex.PipeNode]()).Interface()

	assert.NotNil(t, tmplPipeNode)

	pipeNodeSetter, ok := tmplPipeNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	pipeNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestTemplateNode_Set(t *testing.T) {
	node := templatex.NewNode()
	templateNode := &parse.TemplateNode{
		NodeType: parse.NodeTemplate,
		Pos:      1,
		Line:     1,
		Name:     "template1",
		Pipe: &parse.PipeNode{
			NodeType: parse.NodePipe,
			Pos:      1,
			Line:     1,
			IsAssign: false,
			Decl:     nil,
			Cmds: []*parse.CommandNode{
				{
					NodeType: parse.NodeCommand,
					Pos:      1,
					Args: []parse.Node{
						&parse.FieldNode{
							NodeType: parse.NodeField,
							Pos:      1,
							Ident:    []string{"field1"},
						},
					},
				},
			},
		},
	}

	tmplTemplateNode := reflect.ValueOf(templateNode).Convert(reflect.TypeFor[*templatex.TplNode]()).Interface()

	assert.NotNil(t, tmplTemplateNode)

	templateNodeSetter, ok := tmplTemplateNode.(templatex.FieldsSetter)

	assert.True(t, ok)

	templateNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}
