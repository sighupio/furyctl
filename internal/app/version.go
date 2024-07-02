// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

type VersionsCtn struct {
	Version   string
	GitCommit string
	BuildTime string
	GoVersion string
	OSArch    string
}

func NewVersionsCtn(version, gitCommit, buildTime, goVersion, osArch string) *VersionsCtn {
	return &VersionsCtn{
		Version:   version,
		GitCommit: gitCommit,
		BuildTime: buildTime,
		GoVersion: goVersion,
		OSArch:    osArch,
	}
}
