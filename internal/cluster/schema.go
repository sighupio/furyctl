// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import "strings"

var crSchemaSettings = make(map[string]map[string]SchemaSettings) //nolint:gochecknoglobals, lll // This patterns requires crSchemaSettings

type SchemaSettings interface {
	SchemaPathForPhase(phase string) (string, error)
}

func RegisterSchemaSettings(apiVersion, kind string, settings SchemaSettings) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := crSchemaSettings[lcAPIVersion]; !ok {
		crSchemaSettings[lcAPIVersion] = make(map[string]SchemaSettings)
	}

	crSchemaSettings[lcAPIVersion][lcKind] = settings
}

func GetSchemaSettings(apiVersion, kind string) SchemaSettings {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	return crSchemaSettings[lcAPIVersion][lcKind]
}
