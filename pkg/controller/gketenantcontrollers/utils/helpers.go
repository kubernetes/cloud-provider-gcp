/*
Copyright 2026 The Kubernetes Authors.
*/

package utils

import (
	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
)

const accessLevelLabelKey = "tenancy.gke.io/access-level"
const supervisor = "supervisor"

// IsSupervisor returns true if the ProviderConfig is labeled as a supervisor.
func IsSupervisor(pc *v1.ProviderConfig) bool {
	return pc.Labels[accessLevelLabelKey] == supervisor
}
