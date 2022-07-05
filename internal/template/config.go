// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

type Templates struct {
	Includes        []string `yaml:"includes,omitempty"`
	Excludes        []string `yaml:"excludes,omitempty"`
	Suffix          string   `default:".tmpl" yaml:"suffix,omitempty"`
	ProcessFilename bool     `yaml:"processFilename,omitempty"`
}

type Config struct {
	Data      map[string]map[any]any `yaml:"data,omitempty"`
	Include   map[string]string      `yaml:"include,omitempty"`
	Templates Templates              `yaml:"templates,omitempty"`
}
