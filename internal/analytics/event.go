// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analytics

type Event interface {
	Send(ch chan Event)
	Properties() map[string]interface{}
	Name() string
}

func NewCommandEvent(name, errorMessage string, exitStatus int, details *ClusterDetails) Event {
	props := map[string]interface{}{
		"exitStatus":     exitStatus,
		"errorMessage":   errorMessage,
		"clusterDetails": details,
	}

	return CommandEvent{
		name:       name,
		properties: props,
	}
}

func (c CommandEvent) Send(ch chan Event) {
	ch <- c
}

func (c CommandEvent) Properties() map[string]interface{} {
	return c.properties
}

func (c CommandEvent) Name() string {
	return c.name
}

type CommandEvent struct {
	name string
	properties
}

type properties map[string]interface{}

type ClusterDetails struct {
	Phase      string
	Provider   string
	KFDVersion string
}
