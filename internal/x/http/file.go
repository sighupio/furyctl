// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

var ErrCannotDownloadFile = errors.New("cannot download file")

func DownloadFile(url string) (string, error) {
	out, err := os.CreateTemp(os.TempDir(), "furyctl")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotDownloadFile, err)
	}
	defer out.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotDownloadFile, err)
	}

	resp, err := DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotDownloadFile, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: bad status: %s", ErrCannotDownloadFile, resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotDownloadFile, err)
	}

	return out.Name(), nil
}
