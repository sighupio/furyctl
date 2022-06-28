package merge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/merge"
)

func Test_NewMerger(t *testing.T) {
	a := map[string]any{
		"data": map[string]any{
			"meta": map[string]string{
				"name": "testName",
			},
			"test": map[string]any{
				"testString": "lorem ipsum",
			},
		},
	}

	b := map[string]any{
		"data": map[string]any{
			"meta": map[string]string{
				"name": "testNewName",
				"foo":  "bar",
			},
			"example": "string",
			"test": map[string]any{
				"example": "string",
			},
		},
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(a, ".data.test"),
		merge.NewDefaultModel(b, ".data.test"),
	)

	assert.NotEmpty(t, merger)
}

func Test_Merge(t *testing.T) {
	a := map[string]any{
		"data": map[string]any{
			"meta": map[string]string{
				"name": "testName",
			},
			"test": map[string]any{
				"testString": "lorem ipsum",
			},
		},
	}

	b := map[string]any{
		"data": map[string]any{
			"meta": map[string]string{
				"name": "testNewName",
				"foo":  "bar",
			},
			"example": "string",
			"test": map[string]any{
				"newTestString": "string",
			},
		},
	}

	expectedRes := map[string]any{
		"data": map[string]any{
			"meta": map[string]string{
				"name": "testName",
			},
			"test": map[string]any{
				"newTestString": "string",
				"testString":    "lorem ipsum",
			},
		},
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(a, ".data.test"),
		merge.NewDefaultModel(b, ".data.test"),
	)

	res, err := merger.Merge()

	assert.NoError(t, err)
	assert.NotEmpty(t, res)
	assert.Equal(t, expectedRes, res)
}
