// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analytics

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/dukex/mixpanel"
	"github.com/sirupsen/logrus"
)

const isNext = true

// NewTracker returns a new Tracker instance.
func NewTracker(token, version, arch, os, org, hostname string) *Tracker {
	const timeout = time.Second * 5

	const apiEndpoint = "https://api-eu.mixpanel.com"

	c := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: timeout,
			}).Dial,
			TLSHandshakeTimeout: timeout,
		},
	}

	isDevelopBuild := strings.Contains(version, "develop")

	t := map[string]string{
		"version":      version,
		"origin":       "furyctl",
		"architecture": arch,
		"$os":          os,
		"org":          org,
		"hostname":     hostname,
		"trackID":      getTrackID(token),
		"isNext":       strconv.FormatBool(isNext),
		"development":  strconv.FormatBool(isDevelopBuild),
	}

	tracker := &Tracker{
		client:       mixpanel.NewFromClient(c, token, apiEndpoint),
		trackingInfo: t,
		enable:       true,
		events:       make(chan Event),
	}

	if token == "" {
		tracker.enable = false

		return tracker
	}

	// Start the event processor, this will listen for new tracked events and send them to mixpanel.
	go tracker.processEvents()

	return tracker
}

type Tracker struct {
	trackingInfo
	client mixpanel.Mixpanel
	events chan Event
	enable bool
}

type trackingInfo map[string]string

// Track collects the event to be consumed by the event processor.
func (a *Tracker) Track(event Event) {
	// // add a channel to send events to a goroutine that will send them to mixpanel
	// // this will allow us to send events in a non-blocking way.
	if a.enable {
		a.events <- event
	}
}

// Flush flushes the events queue, guaranteeing that all events are sent to mixpanel.
// This method uses a timeout to send a GuardEvent to the event processor to close the process.
func (a *Tracker) Flush() {
	const timeout = time.Millisecond * 500

	if a.enable {
		go func() {
			time.Sleep(timeout)
			a.events <- NewStopEvent()
		}()

		a.processEvents()

		logrus.Trace("Flushed events queue")
	}
}

// processEvents is the event processor: it will listen for new events and send them to mixpanel.
// This method will stop when a Stop event is received.
func (a *Tracker) processEvents() {
	for {
		e, ok := <-a.events
		if !ok {
			logrus.Trace("Event processor stopped")

			break
		}

		logrus.Trace("Processing event: ", e.Name())

		switch e.(type) {
		case StopEvent:
			logrus.Trace("Stop event received, stopping event processor")
			a.close()

			return

		case CommandEvent:
			logrus.Trace("Sending event: ", e.Name())

			if err := a.sendEvent(e); err != nil {
				logrus.Tracef("failed to send event: %v", err)
			}

			logrus.Trace("Event sent: ", e.Name())
		}
	}
}

// sendEvent sends the event to mixpanel.
func (a *Tracker) sendEvent(event Event) error {
	// Event Properties with machine info.
	p := appendMachineInfo(a.trackingInfo, event.Properties())

	e := &mixpanel.Event{Properties: p}
	if err := a.client.Track(a.trackingInfo["trackID"], event.Name(), e); err != nil {
		return fmt.Errorf("failed to track event: %w", err)
	}

	return nil
}

// Disable disables the tracker.
func (a *Tracker) Disable() {
	a.enable = false

	a.close()
}

// close closes the tracker's event chan.
func (a *Tracker) close() {
	close(a.events)
}

func getTrackID(token string) string {
	if token != "" {
		return token
	}

	return generateMachineID()
}

func generateMachineID() string {
	mid, err := machineid.ProtectedID("furyctl")
	if err != nil {
		logrus.Tracef("failed to generate a machine id: %v", err)

		mid = "na"
	}

	return mid
}

func appendMachineInfo(src map[string]string, dst map[string]any) map[string]any {
	for k, v := range src {
		dst[k] = v
	}

	return dst
}
