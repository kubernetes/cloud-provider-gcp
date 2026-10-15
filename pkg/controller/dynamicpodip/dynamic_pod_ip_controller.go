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
// Package dynamicpodip implements dynamic pod IP allocation for GKE nodes.
//
// The package consists of two cooperating controllers orchestrated by
// starter.go:
//
//   - NodeNetworkConfigStatusController (status_controller.go): The "read"
//     side. Reconciles GCE instance interface alias IP state into
//     NodeNetworkConfig (NNC) custom resource status.
//
//   - NodeNetworkConfigSpecController (spec_controller.go): The "write" side.
//     Evaluates desired IP allocations from nnc.Spec, resolves candidate pod
//     secondary ranges via CandidateRangeProvider (range_provider.go /
//     range_refresher.go), and mutates GCE instance alias IP ranges.
//
// This file defines shared package-level constants and default configuration.
package dynamicpodip

import (
	"fmt"
	"time"
)

const (
	// DefaultBlockSizeMask is the default CIDR mask requested from GCE
	// (e.g. 28 for 16 IPs).
	DefaultBlockSizeMask = 28

	// reconcileTimeout is the maximum time allowed for a single node
	// reconciliation.
	reconcileTimeout = 60 * time.Second
)

var (
	// DefaultBlockSize is the string representation of the default block size
	// (derived from DefaultBlockSizeMask).
	DefaultBlockSize string
	// DefaultCapacity is the number of IPs in the default block size
	// (derived from DefaultBlockSizeMask).
	DefaultCapacity int
)

func init() {
	DefaultCapacity = 1 << (32 - DefaultBlockSizeMask)
	DefaultBlockSize = fmt.Sprintf("/%d", DefaultBlockSizeMask)
}
