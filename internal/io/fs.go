package io

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CheckDirIsEmpty(target string) error {
	if _, err := os.Stat(target); os.IsExist(err) {
		err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			return fmt.Errorf("the target directory is not empty: %s", path)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func CopyBufferToFile(b bytes.Buffer, source, target string) error {
	if strings.TrimSpace(b.String()) == "" {
		fmt.Printf("%s --> resulted in an empty file (%d bytes). Skipping.\n", source, b.Len())
		return nil
	}

	fmt.Printf("%s --> %s\n", source, target)

	destination, err := os.Create(target)
	if err != nil {
		return err
	}

	_, err = b.WriteTo(destination)
	if err != nil {
		return err
	}

	defer destination.Close()

	return nil
}

func CopyFromSourceToTarget(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}

	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}

	defer destination.Close()

	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
