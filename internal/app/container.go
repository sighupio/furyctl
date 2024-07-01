// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"sync"

	"github.com/sighupio/furyctl/internal/analytics"
)

var (
	container *Container      //nolint:gochecknoglobals // singleton pattern.
	lock      = &sync.Mutex{} //nolint:gochecknoglobals // singleton pattern.
)

type Parameters struct {
	TrackerToken    string
	MachineArch     string
	MachineOS       string
	MachineOrg      string
	MachineHostname string
	Version         string
	GitCommit       string
	BuildTime       string
	GoVersion       string
}

type services struct {
	tracker  *analytics.Tracker
	versions *VersionsCtn
}

type Container struct {
	Parameters
	services
}

func NewDefaultParameters() Parameters {
	return Parameters{
		TrackerToken:    "",
		MachineArch:     "unknown",
		MachineOS:       "unknown",
		MachineOrg:      "SIGHUP",
		MachineHostname: "unknown",
		Version:         "unknown",
		GitCommit:       "unknown",
		BuildTime:       "unknown",
		GoVersion:       "unknown",
	}
}

func GetContainerInstance() *Container {
	if container == nil {
		lock.Lock()
		defer lock.Unlock()

		container = &Container{
			Parameters: NewDefaultParameters(),
		}
	}

	return container
}

func (c *Container) Versions() *VersionsCtn {
	if c.versions == nil {
		c.versions = NewVersionsCtn(c.Version, c.GitCommit, c.BuildTime, c.GoVersion, c.MachineArch)
	}

	return c.versions
}

func (c *Container) Tracker() *analytics.Tracker {
	if c.tracker == nil {
		c.tracker = analytics.NewTracker(
			c.TrackerToken,
			c.Version,
			c.MachineArch,
			c.MachineOS,
			c.MachineOrg,
			c.MachineHostname,
		)
	}

	return c.tracker
}
