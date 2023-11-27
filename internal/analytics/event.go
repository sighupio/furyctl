// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analytics

type Event interface {
	Properties() map[string]any
	AddErrorMessage(msg error)
	AddSuccessMessage(msg string)
	AddClusterDetails(c ClusterDetails)
	AddExitCode(code int)
	Name() string
}

func NewCommandEvent(name string) Event {
	return CommandEvent{
		name:       name,
		properties: make(map[string]any),
	}
}

func (c CommandEvent) AddErrorMessage(e error) {
	c.properties["errorMessage"] = e.Error()
	c.properties["success"] = false
}

func (c CommandEvent) AddSuccessMessage(msg string) {
	c.properties["successMessage"] = msg
	c.properties["success"] = true
}

func (c CommandEvent) AddClusterDetails(d ClusterDetails) {
	c.properties["clusterDetails"] = d
}

func (c CommandEvent) AddExitCode(e int) {
	c.properties["exitCode"] = e
}

func (c CommandEvent) Properties() map[string]any {
	return c.properties
}

func (c CommandEvent) Name() string {
	return c.name
}

// NewStopEvent creates a new StopEvent. StopEvent is a special type of event used to close the events processing.
func NewStopEvent() Event {
	return StopEvent{
		name:       "stop",
		properties: make(map[string]any),
	}
}

func (g StopEvent) AddErrorMessage(e error) {
	g.properties["errorMessage"] = e.Error()
}

func (g StopEvent) AddSuccessMessage(msg string) {
	g.properties["successMessage"] = msg
}

func (g StopEvent) AddClusterDetails(d ClusterDetails) {
	g.properties["clusterDetails"] = d
}

func (g StopEvent) AddExitCode(e int) {
	g.properties["exitCode"] = e
}

func (g StopEvent) Properties() map[string]any {
	return g.properties
}

func (g StopEvent) Name() string {
	return g.name
}

// StopEvent is a special event used to close the events processing.
type StopEvent struct {
	name       string
	properties map[string]any
}

type CommandEvent struct {
	name string
	properties
}

type properties map[string]any

type ClusterDetails struct {
	Phase      string
	Provider   string
	KFDVersion string
	DryRun     bool
}
