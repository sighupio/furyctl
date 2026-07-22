// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// AnimationDisabled reports whether live, in-place terminal output has been turned off — either
// explicitly by the operator (--no-tty) or implicitly by debug logging, where raw log lines are
// preferred over an animated repaint. It does not consider whether output is a terminal; callers
// drawing to a specific file should use ShouldAnimate instead.
func AnimationDisabled() bool {
	return NoTTY || logrus.GetLevel() >= logrus.DebugLevel
}

// ShouldAnimate reports whether f can carry animated, in-place terminal output: animation must not
// be disabled (see AnimationDisabled), and f must be a real terminal.
func ShouldAnimate(f *os.File) bool {
	return !AnimationDisabled() && term.IsTerminal(int(f.Fd()))
}
