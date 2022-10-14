// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package merge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/merge"
)

func TestNewDefaultModel(t *testing.T) {
	content := map[any]any{
		"data": map[any]any{
			"meta": map[any]string{
				"name": "example",
			},
			"test": map[any]any{
				"testString": "lorem ipsum",
			},
		},
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	assert.NotEmpty(t, model)
	assert.Equal(t, content, model.Content())
	assert.Equal(t, path, model.Path())
}

func TestNewDefaultModelFromStruct(t *testing.T) {
	type TestSubStruct struct {
		TestOptInt         *int `json:"testOptInt,omitempty"`
		testUnexposed      string
		TestUntaggedString string
	}

	type TestStruct struct {
		TestString      string         `json:"testString"`
		TestOptionalSub *TestSubStruct `json:"testOptionalSub"`
		TestSub         TestSubStruct  `json:"testSub"`
	}

	type TestContent struct {
		Data TestStruct `json:"data"`
	}

	content := TestContent{
		Data: TestStruct{
			TestString: "lorem ipsum",
			TestSub: TestSubStruct{
				TestOptInt:         nil,
				testUnexposed:      "unexposed",
				TestUntaggedString: "untagged",
			},
			TestOptionalSub: nil,
		},
	}

	expectedRes := map[any]any{
		"data": map[any]any{
			"testString": "lorem ipsum",
			"testSub": map[any]any{
				"testOptInt":         nil,
				"TestUntaggedString": "untagged",
			},
			"testOptionalSub": nil,
		},
	}

	path := ".data"

	model := merge.NewDefaultModelFromStruct(content, path, true)

	assert.Equal(t, expectedRes, model.Content())
}

func TestDefaultModel_Content(t *testing.T) {
	content := map[any]any{
		"data": map[any]any{
			"meta": map[any]string{
				"name": "example",
			},
			"test": map[any]any{
				"testString": "lorem ipsum",
			},
		},
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	assert.Equal(t, content, model.Content())
}

func TestDefaultModel_Path(t *testing.T) {
	content := map[any]any{
		"data": map[any]any{
			"meta": map[any]string{
				"name": "example",
			},
			"test": map[any]any{
				"testString": "lorem ipsum",
			},
		},
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	assert.Equal(t, path, model.Path())
}

func TestDefaultModel_Get(t *testing.T) {
	content := map[any]any{
		"data": map[any]any{
			"meta": map[any]string{
				"name": "example",
			},
			"test": map[any]any{
				"testString": "lorem ipsum",
			},
		},
	}

	expectedRes := map[any]any{
		"testString": "lorem ipsum",
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	res, err := model.Get()

	assert.NoError(t, err)
	assert.Equal(t, expectedRes, res)
}

func TestDefaultModel_Walk(t *testing.T) {
	content := map[any]any{
		"data": map[any]any{
			"meta": map[any]string{
				"name": "example",
			},
			"test": map[any]any{
				"testString": "lorem ipsum",
			},
		},
	}

	target := map[any]any{
		"testString":    "lorem ipsum",
		"testNewString": "lorem ipsum new",
	}

	expectedRes := map[any]any{
		"data": map[any]any{
			"meta": map[any]string{
				"name": "example",
			},
			"test": map[any]any{
				"testNewString": "lorem ipsum new",
				"testString":    "lorem ipsum",
			},
		},
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	err := model.Walk(target)

	assert.NoError(t, err)
	assert.Equal(t, expectedRes, model.Content())
}
