/*
Copyright 2024 The Kubernetes Authors.

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
package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cluster/ports"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

const firewallTestTCPTimeout = time.Duration(1 * time.Second)

var _ = Describe("[cloud-provider-gcp-e2e] Firewall Rules", func() {
	f := framework.NewDefaultFramework("firewall-rules")

	var cs clientset.Interface
	BeforeEach(func() {
		cs = f.ClientSet
	})

	AfterEach(func() {
		// After each test
	})

	// Firewall Test
	f.It("control plane should not expose well-known ports", func(ctx context.Context) {
		nodes, err := e2enode.GetReadySchedulableNodes(ctx, cs)
		framework.ExpectNoError(err)

		By("Checking well known ports on master and nodes are not exposed externally")
		nodeAddr := e2enode.FirstAddress(nodes, v1.NodeExternalIP)
		if nodeAddr != "" {
			assertNotReachableHTTPTimeout(nodeAddr, "/", ports.KubeletPort, firewallTestTCPTimeout, false)
			assertNotReachableHTTPTimeout(nodeAddr, "/", ports.KubeletReadOnlyPort, firewallTestTCPTimeout, false)
			assertNotReachableHTTPTimeout(nodeAddr, "/", ports.ProxyStatusPort, firewallTestTCPTimeout, false)
		}

		controlPlaneAddresses := controlPlaneAddresses(ctx, f)
		for _, instanceAddress := range controlPlaneAddresses {
			assertNotReachableHTTPTimeout(instanceAddress, "/healthz", ports.KubeControllerManagerPort, firewallTestTCPTimeout, true)
			assertNotReachableHTTPTimeout(instanceAddress, "/healthz", kubeschedulerconfig.DefaultKubeSchedulerPort, firewallTestTCPTimeout, true)
		}
	})
})

func assertNotReachableHTTPTimeout(ip, path string, port int, timeout time.Duration, enableHTTPS bool) {
	result := PokeHTTP(ip, port, path, &HTTPPokeParams{Timeout: timeout, EnableHTTPS: enableHTTPS})
	if result.Status == HTTPError {
		framework.Failf("Unexpected error checking for reachability of %s:%d: %v", ip, port, result.Error)
	}
	if result.Code != 0 {
		framework.Failf("Was unexpectedly able to reach %s:%d", ip, port)
	}
}

func controlPlaneAddresses(ctx context.Context, f *framework.Framework) []string {
	nodes := framework.GetControlPlaneNodes(ctx, f.ClientSet)
	var ips []string
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				ips = append(ips, addr.Address)
			}
		}
	}
	return ips
}
