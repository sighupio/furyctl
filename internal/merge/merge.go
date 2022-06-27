package merge

import (
	"fmt"
	"strings"
)

type DefaultModel struct {
	content map[string]interface{}
	path    string
}

type Merger struct {
	base   Mergeable
	custom Mergeable
}

type Mergeable interface {
	Get() (map[string]interface{}, error)
	Walk(map[string]interface{}) error
	Content() map[string]interface{}
	Path() string
}

func NewDefaultModel(content map[string]interface{}, path string) *DefaultModel {
	return &DefaultModel{
		content: content,
		path:    path,
	}
}

func NewMerger(b, c Mergeable) *Merger {
	return &Merger{
		base:   b,
		custom: c,
	}
}

func (b *DefaultModel) Content() map[string]interface{} {
	return (*b).content
}

func (b *DefaultModel) Path() string {
	return (*b).path
}

func (b *DefaultModel) Get() (map[string]interface{}, error) {
	ret := (*b).content

	fields := strings.Split((*b).path[1:], ".")

	for _, f := range fields {
		mapAtKey, ok := ret[f]
		if !ok {
			return nil, fmt.Errorf("cannot access key %s on map", f)
		}

		ret, ok = mapAtKey.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("data structure is invalid on key %s", f)
		}
	}

	return ret, nil
}

func (b *DefaultModel) Walk(mergedSection map[string]interface{}) error {
	ret := &b.content

	fields := strings.Split(b.Path()[1:], ".")

	for _, f := range fields {
		_, ok := (*ret)[f]
		if !ok {
			return fmt.Errorf("cannot access key %s on map", f)
		}

		_, ok = (*ret)[f].(map[string]interface{})
		if !ok {
			return fmt.Errorf("data structure is invalid on key %s", f)
		}

		ret = (*ret)[f].(map[string]interface{})
	}

	*ret = mergedSection

	return nil
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
