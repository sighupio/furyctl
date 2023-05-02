// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

func Intersection[T comparable](a, b []T) []T {
	unique := make(map[T]bool, len(a))

	intersection := []T{}

	for _, v := range a {
		unique[v] = true
	}

	for _, w := range b {
		if unique[w] {
			intersection = append(intersection, w)
		}
	}

	return intersection
}
