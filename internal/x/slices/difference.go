// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

import "slices"

// Difference returns the difference between two slices a and b,
// it should contain all the elements that are in a but not in b.
func Difference[T comparable](a, b []T) []T {
	res := make([]T, 0)

	if a == nil {
		return res
	}

	if b == nil {
		return a
	}

	for _, el := range a {
		if !slices.Contains(b, el) {
			res = append(res, el)
		}
	}

	return res
}
