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
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cluster/ports"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	netutils "k8s.io/utils/net"
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

		controlPlaneAddresses := GetControlPlaneAddresses(ctx, cs)
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

// GetControlPlaneAddresses returns all IP addresses on which the kubelet can reach the control plane.
// It may return internal and external IPs, even if we expect for
// e.g. internal IPs to be used (issue #56787), so that we can be
// sure to block the control plane fully during tests.
func GetControlPlaneAddresses(ctx context.Context, c clientset.Interface) []string {
	externalIPs, internalIPs, _ := getControlPlaneAddresses(ctx, c)

	ips := sets.NewString()
	switch framework.TestContext.Provider {
	case "gce":
		for _, ip := range externalIPs {
			ips.Insert(ip)
		}
		for _, ip := range internalIPs {
			ips.Insert(ip)
		}
	default:
		framework.Failf("This test is not supported for provider %s and should be disabled", framework.TestContext.Provider)
	}
	return ips.List()
}

// getControlPlaneAddresses returns the externalIP, internalIP and hostname fields of control plane nodes.
// If any of these is unavailable, empty slices are returned.
func getControlPlaneAddresses(ctx context.Context, c clientset.Interface) ([]string, []string, []string) {
	var externalIPs, internalIPs, hostnames []string

	// Populate the internal IPs.
	eps, err := c.CoreV1().Endpoints(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		framework.Failf("Failed to get kubernetes endpoints: %v", err)
	}
	for _, subset := range eps.Subsets {
		for _, address := range subset.Addresses {
			if address.IP != "" {
				internalIPs = append(internalIPs, address.IP)
			}
		}
	}

	// Populate the external IP/hostname.
	hostURL, err := url.Parse(framework.TestContext.Host)
	if err != nil {
		framework.Failf("Failed to parse hostname: %v", err)
	}
	if netutils.ParseIPSloppy(hostURL.Host) != nil {
		externalIPs = append(externalIPs, hostURL.Host)
	} else {
		hostnames = append(hostnames, hostURL.Host)
	}

	return externalIPs, internalIPs, hostnames
}
