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
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/analytics"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

var (
	version   = "unknown"
	gitCommit = "unknown"
	buildTime = "unknown"
	goVersion = "unknown"
	osArch    = "unknown"
)

func main() {
	versions := map[string]string{
		"version":   version,
		"gitCommit": gitCommit,
		"buildTime": buildTime,
		"goVersion": goVersion,
		"osArch":    osArch,
	}

	logW, err := logFile()
	if err != nil {
		logrus.Fatal(err)
	}

	defer logW.Close()

	h, err := os.Hostname()
	if err != nil {
		logrus.Debug(err)

		h = "unknown"
	}

	t := os.Getenv("FURYCTL_MIXPANEL_TOKEN")
	if t == "" {
		panic("FURYCTL_MIXPANEL_TOKEN environment variable not set")
	}

	// Create the analytics tracker.
	a := analytics.NewTracker(t, versions[version], osArch, runtime.GOOS, "SIGHUP", h)

	defer a.Flush()

	if _, err := cmd.NewRootCommand(versions, logW, a).ExecuteC(); err != nil {
		logrus.Error(err)
	}
}

func logFile() (*os.File, error) {
	// Get the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error while getting current working directory: %w", err)
	}

	// Create the log file.
	logFile, err := os.OpenFile(filepath.Join(cwd, "furyctl.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, iox.RWPermAccess)
	if err != nil {
		return nil, fmt.Errorf("error while creating log file: %w", err)
	}

	// Create the combined writer.
	return logFile, nil
}
