// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

// Map returns a new slice containing the results of applying
// the function f to each element of the original slice.
func Map[T, U any](s []T, f func(T) U) []U {
	res := make([]U, len(s))

	for i, v := range s {
		res[i] = f(v)
	}

	return res
}
