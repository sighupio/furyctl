package merge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/merge"
)

func TestNewDefaultModel(t *testing.T) {
	content := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
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

func TestDefaultModel_Content(t *testing.T) {
	content := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
				"testString": "lorem ipsum",
			},
		},
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	assert.Equal(t, content, model.Content())
}

func TestDefaultModel_Path(t *testing.T) {
	content := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
				"testString": "lorem ipsum",
			},
		},
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	assert.Equal(t, path, model.Path())
}

func TestDefaultModel_Get(t *testing.T) {
	content := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
				"testString": "lorem ipsum",
			},
		},
	}

	expectedRes := map[string]interface{}{
		"testString": "lorem ipsum",
	}

	path := ".data.test"

	model := merge.NewDefaultModel(content, path)

	res, err := model.Get()

	assert.NoError(t, err)
	assert.Equal(t, expectedRes, res)
}

func TestDefaultModel_Walk(t *testing.T) {
	content := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
				"testString": "lorem ipsum",
			},
		},
	}

	target := map[string]interface{}{
		"testString":    "lorem ipsum",
		"testNewString": "lorem ipsum new",
	}

	expectedRes := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
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
