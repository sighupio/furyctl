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
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/app"
)

var (
	version   = "unknown"
	gitCommit = "unknown"
	buildTime = "unknown"
	goVersion = "unknown"
	osArch    = "unknown"
)

var wg sync.WaitGroup

func main() {
	versions := map[string]string{
		"version":   version,
		"gitCommit": gitCommit,
		"buildTime": buildTime,
		"goVersion": goVersion,
		"osArch":    osArch,
	}

	wg.Add(1)
	go checkUpdates(versions["version"])

	if err := cmd.NewRootCommand(versions).Execute(); err != nil {
		logrus.Fatal(err)
	}

	wg.Wait()
}

func checkUpdates(version string) {
	defer wg.Done()

	if version == "unknown" {
		return
	}

	u := app.NewUpdate(version)
	r, err := u.FetchLastRelease()
	if err != nil {
		logrus.Debugf("Error fetching last release: %s", err)
	}

	if r.Version != version {
		logrus.Infof("New furyctl version available: %s --> %s", version, r.Version)
	}
}
