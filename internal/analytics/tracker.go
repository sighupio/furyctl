// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analytics

import (
	"net"
	"net/http"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/dukex/mixpanel"
	"github.com/sirupsen/logrus"
)

const (
	timeout     = time.Second * 5
	APIEndpoint = "https://api-eu.mixpanel.com"
)

func New(token, version, arch, os, org, hostname string) *Tracker {
	c := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: timeout,
			}).Dial,
			TLSHandshakeTimeout: timeout,
		},
	}

	t := map[string]string{
		"version":      version,
		"origin":       "furyctl",
		"architecture": arch,
		"$os":          os,
		"org":          org,
		"hostname":     hostname,
		"trackID":      getTrackID(token),
	}

	return &Tracker{
		client:       mixpanel.NewFromClient(c, token, APIEndpoint),
		enable:       false,
		trackingInfo: t,
	}
}

type Tracker struct {
	enable bool
	trackingInfo
	client mixpanel.Mixpanel
}

type trackingInfo map[string]string

func (a *Tracker) Track(event Event) error {
	// Event Properties with machine info.
	p := appendMachineInfo(a.trackingInfo, event.Properties())

	e := &mixpanel.Event{Properties: p}
	if err := a.client.Track(a.trackingInfo["trackID"], event.Name(), e); err != nil {
		return err
	}

	return nil
}

func (a *Tracker) IsEnabled() bool {
	return a.enable
}

func (a *Tracker) Disable(enable bool) {
	a.enable = enable
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
		logrus.WithError(err).Debug("failed to generate a machine id")

		mid = "na"
	}

	return mid
}

func appendMachineInfo(src trackingInfo, dst map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		dst[k] = v
	}

	return dst
}
