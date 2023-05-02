// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox

import (
	"bufio"
	"strings"
)

type Prompter struct {
	Reader *bufio.Reader
}

func NewPrompter(r *bufio.Reader) *Prompter {
	return &Prompter{
		Reader: r,
	}
}

func (p *Prompter) Ask(w string) bool {
	response, err := p.Reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSuffix(response, "\n")

	return strings.Compare(response, w) == 0
}
