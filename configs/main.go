// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configs

import "embed"

//go:embed patches
//go:embed provisioners
//go:embed upgrades
var Tpl embed.FS
