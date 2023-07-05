// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

func Uniq[T comparable](s []T) []T {
	unique := make(map[T]bool, len(s))
	us := make([]T, 0)

	for _, v := range s {
		if !unique[v] {
			us = append(us, v)
			unique[v] = true
		}
	}

	return us
}
