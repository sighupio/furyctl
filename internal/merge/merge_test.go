package merge_test

import (
	"testing"

	"github.com/sighupio/furyctl/internal/merge"
)

func Test_Deep_Copy(t *testing.T) {
	a := map[string]interface{}{
		"data": map[string]interface{}{
			"meta": map[string]string{
				"name": "pippo",
			},
			"test": []map[string]interface{}{
				{
					"testArray": "lorem ipsum",
				},
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
			"test": []map[string]interface{}{
				{
					"pippolandia": "pippo1",
					"plutolandia": "pluto1",
				},
			},
		},
	}

	res := merge.DeepCopy(a, b)

	t.Log(res)
}
