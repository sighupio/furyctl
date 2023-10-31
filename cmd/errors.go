// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import "errors"

var (
	ErrParsingFlag        = errors.New("error while parsing flag")
	ErrKubeconfigReq      = errors.New("either the KUBECONFIG environment variable or the --kubeconfig flag should be set")
	ErrKubeconfigNotFound = errors.New("kubeconfig file not found")
)
