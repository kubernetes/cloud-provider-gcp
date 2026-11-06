/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamicpodip

// Reasons of the events the dynamic pod IP controllers record on Nodes.
const (
	// PodCIDRRemovedOutOfBandReason is recorded when a pod CIDR that was published
	// in NodeNetworkConfig status disappears from the VM without having been
	// requested for release via spec.releasableCIDRs.
	PodCIDRRemovedOutOfBandReason = "PodCIDRRemovedOutOfBand"
)
