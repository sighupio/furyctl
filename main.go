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
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"strings"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/analytics"
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
	var logFile *os.File

	versions := map[string]string{
		"version":   version,
		"gitCommit": gitCommit,
		"buildTime": buildTime,
		"goVersion": goVersion,
		"osArch":    osArch,
	}

	defer logFile.Close()

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

	// Create the analytics tracker.
	a := analytics.NewTracker(mixPanelToken, versions["version"], osArch, runtime.GOOS, "SIGHUP", h)

	defer a.Flush()

	if _, err := cmd.NewRootCommand(versions, logFile, a, mixPanelToken).ExecuteC(); err != nil {
		log.Error(err)

		return 1
	}

	return 0
}
