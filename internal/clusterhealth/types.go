// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterhealth

// The shapes below mirror the parts of the kubectl JSON output these checks read.

type resourceList struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type containerResources struct {
	Requests resourceList `json:"requests"`
}

type podContainer struct {
	Resources containerResources `json:"resources"`
}

type ownerReference struct {
	Kind string `json:"kind"`
}

type podMetadata struct {
	Name            string           `json:"name"`
	Namespace       string           `json:"namespace"`
	OwnerReferences []ownerReference `json:"ownerReferences"`
}

type podSpec struct {
	NodeName   string         `json:"nodeName"`
	Containers []podContainer `json:"containers"`
}

type containerStatus struct {
	Name         string `json:"name"`
	RestartCount int    `json:"restartCount"`
}

type podCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type podStatus struct {
	Phase             string            `json:"phase"`
	Reason            string            `json:"reason"`
	ContainerStatuses []containerStatus `json:"containerStatuses"`
	Conditions        []podCondition    `json:"conditions"`
}

type podItem struct {
	Metadata podMetadata `json:"metadata"`
	Spec     podSpec     `json:"spec"`
	Status   podStatus   `json:"status"`
}

type podList struct {
	Items []podItem `json:"items"`
}

type workloadMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type workloadSpec struct {
	Replicas *int `json:"replicas"`
}

type workloadStatus struct {
	ReadyReplicas          int `json:"readyReplicas"`
	DesiredNumberScheduled int `json:"desiredNumberScheduled"`
	NumberReady            int `json:"numberReady"`
}

type workloadItem struct {
	Kind     string           `json:"kind"`
	Metadata workloadMetadata `json:"metadata"`
	Spec     workloadSpec     `json:"spec"`
	Status   workloadStatus   `json:"status"`
}

type workloadList struct {
	Items []workloadItem `json:"items"`
}

type pdbStatus struct {
	DisruptionsAllowed int `json:"disruptionsAllowed"`
	CurrentHealthy     int `json:"currentHealthy"`
	DesiredHealthy     int `json:"desiredHealthy"`
}

type pdbItem struct {
	Metadata workloadMetadata `json:"metadata"`
	Status   pdbStatus        `json:"status"`
}

type pdbList struct {
	Items []pdbItem `json:"items"`
}

type nodeCapacityMetadata struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

type nodeCapacityStatus struct {
	Allocatable resourceList `json:"allocatable"`
}

type nodeCapacityItem struct {
	Metadata nodeCapacityMetadata `json:"metadata"`
	Status   nodeCapacityStatus   `json:"status"`
}

type nodeCapacityList struct {
	Items []nodeCapacityItem `json:"items"`
}
