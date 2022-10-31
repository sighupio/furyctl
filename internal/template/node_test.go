// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package template_test

import (
	"reflect"
	"testing"
	"text/template"
	"text/template/parse"

	"github.com/stretchr/testify/assert"

	template2 "github.com/sighupio/furyctl/internal/template"
)

func TestNewNode(t *testing.T) {
	node := template2.NewNode()

	assert.Equal(t, []string{}, node.Fields)
}

func TestNode_FromNodeList(t *testing.T) {
	node := template2.NewNode()
	tmpl := template.Must(template.New("test").Parse("{{.field1}}"))
	node.FromNodeList(tmpl.Root.Nodes)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestActionNode_Set(t *testing.T) {
	node := template2.NewNode()
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

	tmplActionNode := reflect.ValueOf(actionNode).Convert(reflect.TypeOf(&template2.ActionNode{})).Interface()

	assert.NotNil(t, tmplActionNode)

	actionNodeSetter, ok := tmplActionNode.(template2.FieldsSetter)

	assert.True(t, ok)

	actionNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestFieldNode_Set(t *testing.T) {
	node := template2.NewNode()
	fieldNode := &parse.FieldNode{
		NodeType: parse.NodeField,
		Pos:      1,
		Ident:    []string{"field1"},
	}

	tmplFieldNode := reflect.ValueOf(fieldNode).Convert(reflect.TypeOf(&template2.FieldNode{})).Interface()

	assert.NotNil(t, tmplFieldNode)

	fieldNodeSetter, ok := tmplFieldNode.(template2.FieldsSetter)

	assert.True(t, ok)

	fieldNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestIfNode_Set(t *testing.T) {
	node := template2.NewNode()
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

	tmplIfNode := reflect.ValueOf(ifNode).Convert(reflect.TypeOf(&template2.IfNode{})).Interface()

	assert.NotNil(t, tmplIfNode)

	ifNodeSetter, ok := tmplIfNode.(template2.FieldsSetter)

	assert.True(t, ok)

	ifNodeSetter.Set(node)

	assert.Equal(t, []string{".field1", ".field2", ".field3"}, node.Fields)
}

func TestListNode_Set(t *testing.T) {
	node := template2.NewNode()
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

	tmplListNode := reflect.ValueOf(listNode).Convert(reflect.TypeOf(&template2.ListNode{})).Interface()

	assert.NotNil(t, tmplListNode)

	listNodeSetter, ok := tmplListNode.(template2.FieldsSetter)

	assert.True(t, ok)

	listNodeSetter.Set(node)

	assert.Equal(t, []string{".field1", ".field2"}, node.Fields)
}

func TestNode_Set(t *testing.T) {
	node := template2.NewNode()

	node.Set([]string{".field1", ".field2"})

	assert.Equal(t, []string{".field1", ".field2"}, node.Fields)
}

func TestVariableNode_Set(t *testing.T) {
	node := template2.NewNode()
	variableNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Pos:      1,
		Ident:    []string{"variable1"},
	}

	tmplVariableNode := reflect.ValueOf(variableNode).Convert(reflect.TypeOf(&template2.VariableNode{})).Interface()

	assert.NotNil(t, tmplVariableNode)

	variableNodeSetter, ok := tmplVariableNode.(template2.FieldsSetter)

	assert.True(t, ok)

	variableNodeSetter.Set(node)

	assert.Equal(t, []string{".variable1"}, node.Fields)
}

func TestRangeNode_Set(t *testing.T) {
	node := template2.NewNode()
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

	tmplRangeNode := reflect.ValueOf(rangeNode).Convert(reflect.TypeOf(&template2.RangeNode{})).Interface()

	assert.NotNil(t, tmplRangeNode)

	rangeNodeSetter, ok := tmplRangeNode.(template2.FieldsSetter)

	assert.True(t, ok)

	rangeNodeSetter.Set(node)

	assert.Equal(t, []string{".field1", ".field2", ".field3"}, node.Fields)
}

func TestPipeNode_Set(t *testing.T) {
	node := template2.NewNode()
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

	tmplPipeNode := reflect.ValueOf(pipeNode).Convert(reflect.TypeOf(&template2.PipeNode{})).Interface()

	assert.NotNil(t, tmplPipeNode)

	pipeNodeSetter, ok := tmplPipeNode.(template2.FieldsSetter)

	assert.True(t, ok)

	pipeNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}

func TestTemplateNode_Set(t *testing.T) {
	node := template2.NewNode()
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

	tmplTemplateNode := reflect.ValueOf(templateNode).Convert(reflect.TypeOf(&template2.TplNode{})).Interface()

	assert.NotNil(t, tmplTemplateNode)

	templateNodeSetter, ok := tmplTemplateNode.(template2.FieldsSetter)

	assert.True(t, ok)

	templateNodeSetter.Set(node)

	assert.Equal(t, []string{".field1"}, node.Fields)
}
