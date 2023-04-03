// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

type TransformFunc[T comparable] func(T) T

func identity[T comparable](v T) T {
	return v
}

// Disjoint returns true if the two slices have no elements in common.
func Disjoint[T comparable](a, b []T) bool {
	unique := make(map[T]bool, len(a))

	for _, v := range a {
		unique[v] = true
	}

	for _, w := range b {
		if unique[w] {
			return false
		}
	}

	return true
}

// DisjointTransform returns true if the two slices have no elements in common, it takes two
// transform functions that are applied to the elements of each slice before comparing them.
func DisjointTransform[T comparable](a, b []T, transformA, transformB TransformFunc[T]) bool {
	unique := make(map[T]bool, len(a))

	if transformA == nil {
		transformA = identity[T]
	}

	if transformB == nil {
		transformB = identity[T]
	}

	for _, v := range a {
		unique[transformA(v)] = true
	}

	for _, w := range b {
		if unique[transformB(w)] {
			return false
		}
	}

	return true
}
