package merge

import (
	"fmt"
)

type Merger struct {
	base   Mergeable
	custom Mergeable
}

func NewMerger(b, c Mergeable) *Merger {
	return &Merger{
		base:   b,
		custom: c,
	}
}

func (m *Merger) Merge() (map[string]interface{}, error) {
	preparedBase, err := m.base.Get()
	if err != nil {
		return nil, fmt.Errorf("incorrect base file, %s", err.Error())
	}

	preparedCustom, err := m.custom.Get()
	if err != nil {
		return preparedBase, nil
	}

	mergedSection := deepCopy(preparedBase, preparedCustom)

	err = m.base.Walk(mergedSection)

	return m.base.Content(), err

}

func deepCopy(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = deepCopy(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
