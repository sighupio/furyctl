// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io

func NewNullWriter() *NullWriter {
	return &NullWriter{}
}

type NullWriter struct{}

func (nw *NullWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}
