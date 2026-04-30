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
	"encoding/json"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"k8s.io/metis/pkg"
)

func loadNetConf(bytes []byte) (*NetConf, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}
	return conf, nil
}

func getGatewayIP(ipNet *net.IPNet) net.IP {
	if ipNet == nil {
		return nil
	}
	ones, bits := ipNet.Mask.Size()
	if bits-ones < 2 { // smaller than /30 for IPv4, or /126 for IPv6
		if ipNet.IP.To4() != nil {
			return net.ParseIP(pkg.DefaultGatewayIPv4)
		}
		return net.ParseIP(pkg.DefaultGatewayIPv6)
	}

	gw := make(net.IP, len(ipNet.IP))
	copy(gw, ipNet.IP)
	for i := len(gw) - 1; i >= 0; i-- {
		gw[i]++
		if gw[i] > 0 {
			break
		}
	}
	return gw
}

func loadK8sArgs(args string) (*K8sArgs, error) {
	k8sArgs := &K8sArgs{}
	if err := types.LoadArgs(args, k8sArgs); err != nil {
		return nil, fmt.Errorf("failed to parse CNI args: %v", err)
	}
	return k8sArgs, nil
}
