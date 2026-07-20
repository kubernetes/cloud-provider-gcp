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

package daemon

import (
	"context"
	"fmt"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getNodeNetworkConfig fetches the NodeNetworkConfig CR.
// It prefers using the lister (cache) for efficiency, but falls back to the API client
// if the lister is not available. This fallback is primarily to support unit tests
// that do not initialize the full informer stack.
func getNodeNetworkConfig(
	ctx context.Context,
	nncLister nnclisters.NodeNetworkConfigLister,
	nncClient nncclientset.Interface,
	nodeName string,
) (*nncv1.NodeNetworkConfig, error) {
	if nncLister != nil {
		nnc, err := nncLister.Get(nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from lister: %w", err)
		}
		return nnc, nil
	}
	if nncClient != nil {
		nnc, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from API: %w", err)
		}
		return nnc, nil
	}
	return nil, fmt.Errorf("no client or lister available to fetch NodeNetworkConfig")
}
