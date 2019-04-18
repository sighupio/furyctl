package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
)

func MustMkdirInCurrentDirectory(dirname string) {
	dir := MustGetCurrentDir()
	p := filepath.Join(dir, dirname)
	MustMkdir(p)
}

func MustMkdir(dirname string) {
	err := os.Mkdir(dirname, 0700)
	if err != nil {
		panic(err)
	}
}

func MustGetCurrentDir() string {
	p, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return p
}

func SafeWriteFileOrExit(filename string, fileContent []byte) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err := ioutil.WriteFile(filename, fileContent, 0777)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("file %s already exists\n", filename)
		os.Exit(0)
	}
}

func FindBasesFromVendor(vendorPath string) (paths []string) {
	matches, err := doublestar.Glob(filepath.Join(vendorPath, "**", "kustomization.{yaml,yml}"))
	if err != nil {
		panic(err)
	}

	for _, v := range matches {
		s := strings.Replace(v, "/kustomization.yaml", "", 1)
		s = strings.Replace(s, "/kustomization.yml", "", 1)
		paths = append(paths, s)
	}

	return paths
}
