package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

var ErrCannotDownloadFile = fmt.Errorf("cannot download file")

func DownloadFile(url string) (string, error) {
	out, err := os.CreateTemp(os.TempDir(), "furyctl")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCannotDownloadFile, err)
	}
	defer out.Close()

	resp, err := http.Get(url)
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
