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
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/analytics"
)

var (
	version   = "unknown"
	gitCommit = "unknown"
	buildTime = "unknown"
	goVersion = "unknown"
	osArch    = "unknown"

	mixpanelToken = os.Getenv("FURYCTL_MIXPANEL_TOKEN")
)

func main() {
	var logFile *os.File

	versions := map[string]string{
		"version":   version,
		"gitCommit": gitCommit,
		"buildTime": buildTime,
		"goVersion": goVersion,
		"osArch":    osArch,
	}

	rootCmd := cmd.NewRootCommand(versions, logFile)

	defer logW.Close()

	h, err := os.Hostname()
	if err != nil {
		logrus.Debug(err)

		h = "unknown"
	}

	a := analytics.New(mixpanelToken, versions[version], osArch, runtime.GOOS, "SIGHUP", h)

	if executedCmd, err := cmd.NewRootCommand(versions, logW, a).ExecuteC(); err != nil {
		if a.IsEnabled() {
			if err := a.Track(analytics.NewCommandEvent(getCmdFullname(executedCmd), err.Error(), 1, nil)); err != nil {
				logrus.Debug(err)
			}
		}

		logrus.Fatal(err)
	}

	rootCmd := cmd.NewRootCommand(versions, logW, a)
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
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

// getCmdFullname returns the full name of the command.
func getCmdFullname(cmd *cobra.Command) string {
	if cmd.Parent() == nil || cmd.Parent().Name() == "furyctl" {
		return cmd.Name()
	}

	return fmt.Sprintf("%s %s", getCmdFullname(cmd.Parent()), cmd.Name())
}
