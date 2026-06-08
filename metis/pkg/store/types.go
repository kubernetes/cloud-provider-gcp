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

package store

import (
	"errors"
)

var (
	// ErrCidrAlreadyExists is returned when a CIDR block already exists in the store.
	ErrCidrAlreadyExists = errors.New("cidr block already exists")

	// ErrNoAvailableIPs is returned when no available IPs can be found in any CIDR block.
	ErrNoAvailableIPs = errors.New("no available IPs in store")

	// ErrCidrBlockExhausted is returned when an IPv6 CIDR block cannot be expanded further.
	ErrCidrBlockExhausted = errors.New("ipv6 cidr block exhausted and cannot be expanded")
)

// IPFamily represents the IP protocol family.
type IPFamily string

const (
	IPv4 IPFamily = "ipv4"
	IPv6 IPFamily = "ipv6"
)

type CidrBlockState string

const (
	StateReady    CidrBlockState = "Ready"
	StateDraining CidrBlockState = "Draining"
	StateDeleting CidrBlockState = "Deleting"
)
