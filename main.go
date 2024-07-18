// Copyright © 2017-present SIGHUP SRL support@sighup.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/app"
)

var (
	version       = "unknown"
	gitCommit     = "unknown"
	buildTime     = "unknown"
	goVersion     = "unknown"
	osArch        = "unknown"
	mixPanelToken = ""
)

func main() {
	os.Exit(exec())
}

func exec() int {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go checkNewRelease(wg, version)

	log := &logrus.Logger{
		Out: os.Stdout,
		Formatter: &logrus.TextFormatter{
			ForceColors:      true,
			DisableTimestamp: true,
		},
		Level: logrus.DebugLevel,
	}

	h, err := os.Hostname()
	if err != nil {
		log.Debug(err)

		h = "unknown"
	}

	mixPanelToken = strings.ReplaceAll(mixPanelToken, "\"", "")
	mixPanelToken = strings.ReplaceAll(mixPanelToken, "'", "")

	ctn := app.GetContainerInstance()

	ctn.Version = version
	ctn.GitCommit = gitCommit
	ctn.BuildTime = buildTime
	ctn.GoVersion = goVersion
	ctn.MachineArch = osArch
	ctn.MachineOS = runtime.GOOS
	ctn.TrackerToken = mixPanelToken
	ctn.MachineHostname = h

	defer wg.Wait()

	if _, err := cmd.NewRootCmd().ExecuteC(); err != nil {
		log.Error(err)

		return 1
	}

	return 0
}

func checkNewRelease(wg *sync.WaitGroup, v string) {
	defer wg.Done()

	newRel, err := app.CheckNewRelease(v)
	if err != nil {
		logrus.Trace(err)

		return
	}

	if newRel != "" {
		logrus.Infof("There is a newer release available: %s", newRel)

		return
	}
}
