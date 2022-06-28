// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analytics

import (
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/dukex/mixpanel"
	log "github.com/sirupsen/logrus"
)

const (
	mixpanelToken = "07964a709c19657ded1d402e5f5469b2"

	bootstrapInitEvent    = "BootstrapInit"
	bootstrapApplyEvent   = "BootstrapApply"
	bootstrapDestroyEvent = "BootstrapDestroy"
	clusterInitEvent      = "ClusterInit"
	clusterApplyEvent     = "ClusterApply"
	clusterDestroyEvent   = "ClusterDestroy"
)

var (
	mixpanelClient mixpanel.Mixpanel
	disable        bool
	version        string
)

func enabled() bool {
	return !disable
}

//Version will set the version of the CLI
func Version(v string) {
	version = v
}

//Disable will disable analytics
func Disable(d bool) {
	disable = d
}

func init() {
	c := &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	mixpanelClient = mixpanel.NewFromClient(c, mixpanelToken, "https://api-eu.mixpanel.com")
}

func track(event string, success bool, token string, props map[string]interface{}) {
	if enabled() {
		mpOS := ""
		switch runtime.GOOS {
		case "darwin":
			mpOS = "Mac OS X"
		case "windows":
			mpOS = "Windows"
		case "linux":
			mpOS = "Linux"
		}

		origin := "furyctl"

		if props == nil {
			props = map[string]interface{}{}
		}

		props["$os"] = mpOS
		props["version"] = version
		props["origin"] = origin
		props["success"] = success

		e := &mixpanel.Event{Properties: props}
		trackID := getTrackID(token)
		if err := mixpanelClient.Track(trackID, event, e); err != nil {
			log.WithError(err).Debugf("Failed to send analytics: %s", err)
		}
	} else {
		log.Debugf("not sending event for %s", event)
	}
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
		log.WithError(err).Debug("failed to generate a machine id")
		mid = "na"
	}

	return mid
}

// TrackBootstrapInit sends a tracking event to mixpanel when the user uses the bootstrap init command
func TrackBootstrapInit(token string, success bool, provisioner string) {
	props := map[string]interface{}{
		"provisioner": provisioner,
		"githubToken": token,
	}
	track(bootstrapInitEvent, success, token, props)
}

// TrackBootstrapApply sends a tracking event to mixpanel when the user uses the bootstrap update command
func TrackBootstrapApply(token string, success bool, provisioner string, dryRun bool) {
	props := map[string]interface{}{
		"provisioner": provisioner,
		"dryRun":      dryRun,
		"githubToken": token,
	}
	track(bootstrapApplyEvent, success, token, props)
}

// TrackBootstrapDestroy sends a tracking event to mixpanel when the user uses the bootstrap destroy command
func TrackBootstrapDestroy(token string, success bool, provisioner string) {
	props := map[string]interface{}{
		"provisioner": provisioner,
		"githubToken": token,
	}
	track(bootstrapDestroyEvent, success, token, props)
}

// TrackClusterInit sends a tracking event to mixpanel when the user uses the cluster init command
func TrackClusterInit(token string, success bool, provisioner string) {
	props := map[string]interface{}{
		"provisioner": provisioner,
		"githubToken": token,
	}
	track(clusterInitEvent, success, token, props)
}

// TrackClusterApply sends a tracking event to mixpanel when the user uses the cluster update command
func TrackClusterApply(token string, success bool, provisioner string, dryRun bool) {
	props := map[string]interface{}{
		"provisioner": provisioner,
		"dryRun":      dryRun,
		"githubToken": token,
	}
	track(clusterApplyEvent, success, token, props)
}

// TrackClusterDestroy sends a tracking event to mixpanel when the user uses the cluster destroy command
func TrackClusterDestroy(token string, success bool, provisioner string) {
	props := map[string]interface{}{
		"provisioner": provisioner,
		"githubToken": token,
	}
	track(clusterDestroyEvent, success, token, props)
}
