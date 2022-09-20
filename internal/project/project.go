// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package project

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/pkg/utils"
)

const (
	pathAlreadyExistsErr  = "Directory already exists"
	pathCreationErr       = "Path dir couldn't be created. %v"
	defaultDirPermission  = 0o700
	defaultFilePermission = 0o600
)

// Project represents a simple structure with a couple of useful methods to init a project
type Project struct {
	Path string
}

// Reset deletes and recreates the (p.Path) base directory
func (p *Project) Reset() (err error) {
	_, err = os.Stat(p.Path)
	if !os.IsNotExist(err) { // Exists
		logrus.Warnf("Removing %v directory", p.Path)
		err = os.RemoveAll(p.Path)
		if err != nil {
			logrus.Errorf("Error removing the base dir %v. %v", p.Path, err)
			return err
		}
	}
	return nil
}

// CreateSubDirs creates directories inside the p.Path base directory
func (p *Project) CreateSubDirs(subDirs []string) (err error) {
	_, err = os.Stat(p.Path)
	if !os.IsNotExist(err) {
		logrus.Error(pathAlreadyExistsErr)
		return errors.New(pathAlreadyExistsErr)
	}
	if os.IsNotExist(err) {
		for _, subDir := range subDirs {
			err = os.MkdirAll(fmt.Sprintf("%v/%v", p.Path, subDir), defaultDirPermission)
			if err != nil {
				logrus.Errorf(pathCreationErr, err)
				return err
			}
		}
	}
	return nil
}

// WriteFile writes a new file (fileName) with the content specified
func (p *Project) WriteFile(fileName string, content []byte) (err error) {
	filePath := fmt.Sprintf("%v/%v", p.Path, fileName)
	err = utils.EnsureDir(filePath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, content, os.FileMode(defaultFilePermission))
}

// Check if the project directory exists.
// TODO improve the checks
func (p *Project) Check() error {
	_, err := os.Stat(p.Path)
	if os.IsNotExist(err) {
		logrus.Errorf("Directory does not exists. %v", err)
		return errors.New("Directory does not exists")
	}
	return nil
}
