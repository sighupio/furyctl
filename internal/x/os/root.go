// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package osx

import (
	"errors"
	"fmt"
	"os/user"
)

var ErrCannotGetCurrentUser = errors.New("cannot get current user")

func IsRoot() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrCannotGetCurrentUser, err)
	}

	return u.Uid == "0", nil
}
