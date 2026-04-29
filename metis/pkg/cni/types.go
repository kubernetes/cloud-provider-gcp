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

package cni

import (
	"github.com/containernetworking/cni/pkg/types"
	"google.golang.org/grpc"
	pb "k8s.io/metis/api/adaptiveipam/v1"
)

// IPAM extends standard CNI IPAM configuration.
type IPAM struct {
	types.IPAM
	Ranges [][]struct {
		Subnet string `json:"subnet"`
	} `json:"ranges,omitempty"`
}

// NetConf extends standard CNI network configuration.
type NetConf struct {
	types.NetConf
	IPAM         IPAM   `json:"ipam,omitempty"`
}

// K8sArgs contains the standard Kubernetes CNI arguments.
type K8sArgs struct {
	types.CommonArgs
	K8S_POD_NAME      types.UnmarshallableString `json:"K8S_POD_NAME"`
	K8S_POD_NAMESPACE types.UnmarshallableString `json:"K8S_POD_NAMESPACE"`
}

type Plugin struct {
	newClientFunc func(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error)
	socketPath    string
	logFile       string
}
