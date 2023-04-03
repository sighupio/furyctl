// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

// Clean removes all zero values from a slice.
func Clean[T comparable](slice []T) []T {
	result := make([]T, 0)

	zeroValue := *new(T)

	for _, v := range slice {
		if v != zeroValue {
			result = append(result, v)
		}
	}

	return result
}
