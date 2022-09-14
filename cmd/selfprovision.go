// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sighupio/furyctl/internal/configuration"
	"github.com/sighupio/furyctl/internal/project"
	"github.com/sirupsen/logrus"
)

var (
	stop chan os.Signal

	prj *project.Project
	cfg *configuration.Configuration
)

func parseConfig(path string, kind string) (err error) {
	logrus.Debugf("parsing configuration file %v", path)
	cfg, err = configuration.Parse(path)
	if err != nil {
		logrus.Errorf("error parsing configuration file: %v", err)
		return err
	}
	if cfg.Kind != kind {
		errMessage := fmt.Sprintf("error parsing configuration file. Unexpected kind. Got: %v but: %v expected", cfg.Kind, kind)
		logrus.Error(errMessage)
		return errors.New(errMessage)
	}
	return nil
}

func warning(command string) {
	fmt.Printf(`
  Forced stop of the %v process.
  furyctl can not guarantee the correct behavior after this
  action. Try to recover the state of the process running:

  $ furyctl %v update

`, command, command)
}

func handleStopSignal(command string, c chan os.Signal) {
	go func() {
		<-c
		fmt.Println("\r  Are you sure you want to stop it?\n  Write 'yes' to force close it. Press enter to continue")
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil {
			os.Exit(2)
		}
		text = strings.ReplaceAll(text, "\n", "")
		if strings.Compare("yes", text) == 0 {
			warning(command)
			os.Exit(1)
		}
		handleStopSignal(command, c)
	}()
}
