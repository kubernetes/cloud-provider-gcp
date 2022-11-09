/*
Copyright 2017 The Kubernetes Authors.

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

package gce

import (
	"context"

	compute "google.golang.org/api/compute/v1"
)

// GetSubnetwork returns a compute.Network associated with the subnetwork used
// for this client.
func (g *Cloud) getSubnetwork(ctx context.Context, projectName, region, subnetworkName string) (*compute.Subnetwork, error) {

	subnetwork, err := g.service.Subnetworks.Get(projectName, region, subnetworkName).Context(ctx).Do()

	return subnetwork, err

}
