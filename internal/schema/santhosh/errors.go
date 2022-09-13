package santhosh

import (
	"errors"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema"
	"golang.org/x/exp/slices"
)

var ErrObjTypeAssertion = errors.New("obj type assertion failed")

func JoinPtrPath(path []any) string {
	strpath := "#"

	for _, key := range path {
		switch v := key.(type) {
		case int:
			strpath += "/" + strconv.Itoa(v)

		case string:
			strpath += "/" + v
		}
	}

	return strpath
}

func GetValueAtPath(obj any, path []any) (any, error) {
	for _, key := range path {
		switch v := key.(type) {
		case int:
			tobj, ok := obj.([]any)
			if !ok {
				return nil, ErrObjTypeAssertion
			}

			obj = tobj[v]

		case string:
			tobj, ok := obj.(map[string]any)
			if !ok {
				return nil, ErrObjTypeAssertion
			}

			obj = tobj[v]
		}
	}

	return obj, nil
}

func GetPtrPaths(err error) [][]any {
	var terr *jsonschema.ValidationError

	if errors.As(err, &terr) {
		ptrs := extractPtrs(terr)

		mptrs := minimizePtrs(ptrs)

		return explodePtrs(mptrs)
	}

	return nil
}

func extractPtrs(err *jsonschema.ValidationError) []string {
	ptrs := []string{err.InstancePtr}

	for _, cause := range err.Causes {
		if len(cause.Causes) > 0 {
			ptrs = append(ptrs, extractPtrs(cause)...)
		} else {
			ptrs = append(ptrs, cause.InstancePtr)
		}
	}

	return ptrs
}

func minimizePtrs(ptrs []string) []string {
	slices.Sort(ptrs)

	return slices.Compact(ptrs)
}

func explodePtrs(ptrs []string) [][]any {
	eptrs := make([][]any, len(ptrs))
	for i, p := range ptrs {
		eptrs[i] = explodePtr(strings.TrimLeft(p, "#/"))
	}

	return eptrs
}

func explodePtr(ptr string) []any {
	parts := strings.Split(ptr, "/")
	ptrParts := make([]any, len(parts))

	for i, part := range parts {
		if numpart, err := strconv.Atoi(part); err == nil {
			ptrParts[i] = numpart
		} else {
			ptrParts[i] = part
		}
	}

	return ptrParts
}
