package merge_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/merge"
)

func Test_Merge(t *testing.T) {
	a := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": map[string]interface{}{
				"testArray": "lorem ipsum",
			},
		},
	}

	b := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo2",
				"foo":  "bar",
			},
			"lollo": "pippo",
			"test": map[string]interface{}{
				"pippolandia": "pippo1",
			},
		},
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(a, ".data.test"),
		merge.NewDefaultModel(b, ".data.test"),
	)

	res, _ := merger.Merge()

	t.Log(res)
	t.Log(merger)
}
