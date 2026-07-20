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

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnctypedv1 "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/typed/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type mockNodeNetworkConfigInterface struct {
	nnctypedv1.NodeNetworkConfigInterface
	getFunc    func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error)
	updateFunc func(ctx context.Context, nnc *nncv1.NodeNetworkConfig, opts metav1.UpdateOptions) (*nncv1.NodeNetworkConfig, error)
	patchFunc  func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error)
}

func (m *mockNodeNetworkConfigInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, name, opts)
	}
	return nil, nil
}

func (m *mockNodeNetworkConfigInterface) Update(ctx context.Context, nnc *nncv1.NodeNetworkConfig, opts metav1.UpdateOptions) (*nncv1.NodeNetworkConfig, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, nnc, opts)
	}
	return nil, nil
}

func (m *mockNodeNetworkConfigInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
	if m.patchFunc != nil {
		return m.patchFunc(ctx, name, pt, data, opts, subresources...)
	}
	return nil, nil
}

type mockNetworkingV1 struct {
	nnctypedv1.NetworkingV1Interface
	nncInterface nnctypedv1.NodeNetworkConfigInterface
}

func (m *mockNetworkingV1) NodeNetworkConfigs() nnctypedv1.NodeNetworkConfigInterface {
	return m.nncInterface
}

type mockClientset struct {
	nncclientset.Interface
	networkingV1 nnctypedv1.NetworkingV1Interface
}

func (m *mockClientset) NetworkingV1() nnctypedv1.NetworkingV1Interface {
	return m.networkingV1
}
