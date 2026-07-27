// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

func SetPropertyValue[T any](value any, target *T) {
	if v, ok := value.(T); ok {
		*target = v
	}
}
