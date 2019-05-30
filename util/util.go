package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
)

func CreateFolderInCurrentDirectory(dirname string) {
	currentDir, _ := os.Getwd()
	p := filepath.Join(currentDir, dirname)
	err := os.Mkdir(p, 0700)
	if os.IsExist(err) {
		fmt.Printf("skipping creation of folder '%s' because it already exists \n", dirname)
	}
}

func SafeWriteFileOrExit(filename string, fileContent []byte) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err := ioutil.WriteFile(filename, fileContent, 0777)
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("skipping creation of file %s because it already exists\n", filename)
		if filename == ".gitignore" {
			fmt.Println(`add these enties on your .gitignore manually
			*.retry
			.terraform
			*.tfstate
			*.backup
			`)
		}
		os.Exit(0)
	}
	return nil
}

func FindBasesFromVendor(vendorPath string) (paths []string, err error) {
	matches, err := doublestar.Glob(filepath.Join(vendorPath, "**", "kustomization.{yaml,yml}"))
	if err != nil {
		return nil, err
	}

	for _, v := range matches {
		s := strings.Replace(v, "/kustomization.yaml", "", 1)
		s = strings.Replace(s, "/kustomization.yml", "", 1)
		paths = append(paths, s)
	}

	return paths, nil
}
