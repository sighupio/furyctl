// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox

import (
	"errors"
	"fmt"
	"io"

	bytesx "github.com/sighupio/furyctl/internal/x/bytes"
)

var ErrWriterTransform = errors.New("writer transform error")

type WriterTransform struct {
	W          io.Writer
	Transforms []bytesx.TransformFunc
}

type multiWriterTransform struct {
	writers []WriterTransform
}

func (m *multiWriterTransform) Write(p []byte) (int, error) {
	var err error

	for _, w := range m.writers {
		s := p

		for _, transform := range w.Transforms {
			s, err = transform(s)
			if err != nil {
				return 0, fmt.Errorf("%w: %v", ErrWriterTransform, err)
			}
		}

		n, err := w.W.Write(s)
		if err != nil {
			return n, fmt.Errorf("%w: %v", ErrWriterTransform, err)
		}

		if n != len(s) {
			return n, io.ErrShortWrite
		}
	}

	return len(p), nil
}

func MultiWriterTransform(writers ...WriterTransform) io.Writer {
	return &multiWriterTransform{writers}
}
