// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package drydock

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser asks the desktop to open url. Failing is not fatal: the URL is also logged.
func OpenBrowser(ctx context.Context, url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)

	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)

	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}

	return nil
}
