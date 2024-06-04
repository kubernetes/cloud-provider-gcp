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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"google.golang.org/api/compute/v1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	gcecloud "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/kubernetes/pkg/cluster/ports"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

const (
	firewallTestTCPTimeout = time.Duration(1 * time.Second)
	// Set ports outside of 30000-32767, 80 and 8080 to avoid being allowlisted by the e2e cluster
	firewallTestHTTPPort = int32(29999)
	firewallTestUDPPort  = int32(29998)
)

var _ = Describe("[cloud-provider-gcp-e2e] Firewall Test", func() {
	f := framework.NewDefaultFramework("firewall-test")

	var cs clientset.Interface
	var cloudConfig framework.CloudConfig
	var gceCloud *gcecloud.Cloud
	BeforeEach(func() {
		var err error
		cs = f.ClientSet
		cloudConfig = framework.TestContext.CloudConfig
		gceCloud, err = GetGCECloud()
		framework.ExpectNoError(err)
	})

	// This test takes around 6 minutes to run
	f.It(f.WithSlow(), f.WithSerial(), "should create valid firewall rules for LoadBalancer type service", func(ctx context.Context) {
		ns := f.Namespace.Name
		// This source ranges is just used to examine we have exact same things on LB firewall rules
		firewallTestSourceRanges := []string{"0.0.0.0/1", "128.0.0.0/1"}
		serviceName := "firewall-test-loadbalancer"

		jig := e2eservice.NewTestJig(cs, ns, serviceName)
		nodeList, err := e2enode.GetBoundedReadySchedulableNodes(ctx, cs, e2eservice.MaxNodesForEndpointsTests)
		framework.ExpectNoError(err)

		nodesNames := []string{}
		for _, node := range nodeList.Items {
			nodesNames = append(nodesNames, node.Name)
		}
		nodesSet := sets.NewString(nodesNames...)

		By("Creating a LoadBalancer type service with ExternalTrafficPolicy=Global")
		svc, err := jig.CreateLoadBalancerService(ctx, e2eservice.GetServiceLoadBalancerCreationTimeout(ctx, cs), func(svc *v1.Service) {
			svc.Spec.Ports = []v1.ServicePort{{Protocol: v1.ProtocolTCP, Port: firewallTestHTTPPort}}
			svc.Spec.LoadBalancerSourceRanges = firewallTestSourceRanges
		})
		framework.ExpectNoError(err)

		// This configmap is guaranteed to exist after a Loadbalancer type service is created
		By("Getting cluster ID")
		clusterID, err := GetClusterID(ctx, cs)
		framework.ExpectNoError(err)
		framework.Logf("Got cluster ID: %v", clusterID)

		defer func() {
			_, err = jig.UpdateService(ctx, func(svc *v1.Service) {
				svc.Spec.Type = v1.ServiceTypeNodePort
				svc.Spec.LoadBalancerSourceRanges = nil
			})
			framework.ExpectNoError(err)
			err = cs.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
			By("Waiting for the local traffic health check firewall rule to be deleted")
			localHCFwName := MakeHealthCheckFirewallNameForLBService(clusterID, cloudprovider.DefaultLoadBalancerName(svc), false)
			_, err := WaitForFirewallRule(ctx, gceCloud, localHCFwName, false, e2eservice.LoadBalancerCleanupTimeout)
			framework.ExpectNoError(err)
		}()
		svcExternalIP := svc.Status.LoadBalancer.Ingress[0].IP

		By("Checking if service's firewall rule is correct")
		lbFw := ConstructFirewallForLBService(svc, cloudConfig.NodeTag)
		fw, err := gceCloud.GetFirewall(lbFw.Name)
		framework.ExpectNoError(err)
		err = VerifyFirewallRule(fw, lbFw, cloudConfig.Network, false)
		framework.ExpectNoError(err)

		By("Checking if service's nodes health check firewall rule is correct")
		nodesHCFw := ConstructHealthCheckFirewallForLBService(clusterID, svc, cloudConfig.NodeTag, true)
		fw, err = gceCloud.GetFirewall(nodesHCFw.Name)
		framework.ExpectNoError(err)
		err = VerifyFirewallRule(fw, nodesHCFw, cloudConfig.Network, false)
		framework.ExpectNoError(err)

		// OnlyLocal service is needed to examine which exact nodes the requests are being forwarded to by the Load Balancer on GCE
		By("Updating LoadBalancer service to ExternalTrafficPolicy=Local")
		svc, err = jig.UpdateService(ctx, func(svc *v1.Service) {
			svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyLocal
		})
		framework.ExpectNoError(err)

		By("Waiting for the nodes health check firewall rule to be deleted")
		_, err = WaitForFirewallRule(ctx, gceCloud, nodesHCFw.Name, false, e2eservice.LoadBalancerCleanupTimeout)
		framework.ExpectNoError(err)

		By("Waiting for the correct local traffic health check firewall rule to be created")
		localHCFw := ConstructHealthCheckFirewallForLBService(clusterID, svc, cloudConfig.NodeTag, false)
		fw, err = WaitForFirewallRule(ctx, gceCloud, localHCFw.Name, true, e2eservice.GetServiceLoadBalancerCreationTimeout(ctx, cs))
		framework.ExpectNoError(err)
		err = VerifyFirewallRule(fw, localHCFw, cloudConfig.Network, false)
		framework.ExpectNoError(err)

		By(fmt.Sprintf("Creating netexec pods on at most %v nodes", e2eservice.MaxNodesForEndpointsTests))
		for i, nodeName := range nodesNames {
			podName := fmt.Sprintf("netexec%v", i)

			framework.Logf("Creating netexec pod %q on node %v in namespace %q", podName, nodeName, ns)
			pod := e2epod.NewAgnhostPod(ns, podName, nil, nil, nil,
				"netexec",
				fmt.Sprintf("--http-port=%d", firewallTestHTTPPort),
				fmt.Sprintf("--udp-port=%d", firewallTestUDPPort))
			pod.ObjectMeta.Labels = jig.Labels
			nodeSelection := e2epod.NodeSelection{Name: nodeName}
			e2epod.SetNodeSelection(&pod.Spec, nodeSelection)
			pod.Spec.HostNetwork = true
			_, err := cs.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			framework.ExpectNoError(e2epod.WaitTimeoutForPodReadyInNamespace(ctx, f.ClientSet, podName, f.Namespace.Name, framework.PodStartTimeout))
			framework.Logf("Netexec pod %q in namespace %q running", podName, ns)

			defer func() {
				framework.Logf("Cleaning up the netexec pod: %v", podName)
				err = cs.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()
		}

		// Send requests from outside of the cluster because internal traffic is allowlisted
		By("Accessing the external service ip from outside, all non-master nodes should be reached")
		err = testHitNodesFromOutside(svcExternalIP, firewallTestHTTPPort, e2eservice.GetServiceLoadBalancerPropagationTimeout(ctx, cs), nodesSet)
		framework.ExpectNoError(err)

		// Check if there are overlapping tags on the firewall that extend beyond just the vms in our cluster
		// by removing the tag on one vm and make sure it doesn't get any traffic. This is an imperfect
		// simulation, we really want to check that traffic doesn't reach a vm outside the GKE cluster, but
		// that's much harder to do in the current e2e framework.
		By(fmt.Sprintf("Removing tags from one of the nodes: %v", nodesNames[0]))
		nodesSet.Delete(nodesNames[0])
		// Instance could run in a different zone in multi-zone test. Figure out which zone
		// it is in before proceeding.
		zone := cloudConfig.Zone
		if zoneInLabel, ok := nodeList.Items[0].Labels[v1.LabelFailureDomainBetaZone]; ok {
			zone = zoneInLabel
		} else if zoneInLabel, ok := nodeList.Items[0].Labels[v1.LabelTopologyZone]; ok {
			zone = zoneInLabel
		}
		removedTags := SetInstanceTags(cloudConfig, nodesNames[0], zone, []string{})
		defer func() {
			By("Adding tags back to the node and wait till the traffic is recovered")
			nodesSet.Insert(nodesNames[0])
			SetInstanceTags(cloudConfig, nodesNames[0], zone, removedTags)
			// Make sure traffic is recovered before exit
			err = testHitNodesFromOutside(svcExternalIP, firewallTestHTTPPort, e2eservice.GetServiceLoadBalancerPropagationTimeout(ctx, cs), nodesSet)
			framework.ExpectNoError(err)
		}()

		By("Accessing service through the external ip and examine got no response from the node without tags")
		err = testHitNodesFromOutsideWithCount(svcExternalIP, firewallTestHTTPPort, e2eservice.GetServiceLoadBalancerPropagationTimeout(ctx, cs), nodesSet, 15)
		framework.ExpectNoError(err)
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

		controlPlaneAddresses := framework.GetControlPlaneAddresses(ctx, cs)
		for _, instanceAddress := range controlPlaneAddresses {
			assertNotReachableHTTPTimeout(instanceAddress, "/healthz", ports.KubeControllerManagerPort, firewallTestTCPTimeout, true)
			assertNotReachableHTTPTimeout(instanceAddress, "/healthz", kubeschedulerconfig.DefaultKubeSchedulerPort, firewallTestTCPTimeout, true)
		}
	})
})

// testHitNodesFromOutside checks HTTP connectivity from outside.
func testHitNodesFromOutside(externalIP string, httpPort int32, timeout time.Duration, expectedHosts sets.String) error {
	return testHitNodesFromOutsideWithCount(externalIP, httpPort, timeout, expectedHosts, 1)
}

// testHitNodesFromOutsideWithCount checks HTTP connectivity from outside with count.
func testHitNodesFromOutsideWithCount(externalIP string, httpPort int32, timeout time.Duration, expectedHosts sets.String,
	countToSucceed int) error {
	framework.Logf("Waiting up to %v for satisfying expectedHosts for %v times", timeout, countToSucceed)
	hittedHosts := sets.NewString()
	count := 0
	condition := func() (bool, error) {
		result := e2enetwork.PokeHTTP(externalIP, int(httpPort), "/hostname", &e2enetwork.HTTPPokeParams{Timeout: 1 * time.Second})
		if result.Status != e2enetwork.HTTPSuccess {
			return false, nil
		}

		hittedHost := strings.TrimSpace(string(result.Body))
		if !expectedHosts.Has(hittedHost) {
			framework.Logf("Error hitting unexpected host: %v, reset counter: %v", hittedHost, count)
			count = 0
			return false, nil
		}
		if !hittedHosts.Has(hittedHost) {
			hittedHosts.Insert(hittedHost)
			framework.Logf("Missing %+v, got %+v", expectedHosts.Difference(hittedHosts), hittedHosts)
		}
		if hittedHosts.Equal(expectedHosts) {
			count++
			if count >= countToSucceed {
				return true, nil
			}
		}
		return false, nil
	}

	if err := wait.Poll(time.Second, timeout, condition); err != nil {
		return fmt.Errorf("error waiting for expectedHosts: %v, hittedHosts: %v, count: %v, expected count: %v",
			expectedHosts, hittedHosts, count, countToSucceed)
	}
	return nil
}

func assertNotReachableHTTPTimeout(ip, path string, port int, timeout time.Duration, enableHTTPS bool) {
	result := PokeHTTP(ip, port, path, &HTTPPokeParams{Timeout: timeout, EnableHTTPS: enableHTTPS})
	if result.Status == HTTPError {
		framework.Failf("Unexpected error checking for reachability of %s:%d: %v", ip, port, result.Error)
	}
	if result.Code != 0 {
		framework.Failf("Was unexpectedly able to reach %s:%d", ip, port)
	}
}

// MakeFirewallNameForLBService return the expected firewall name for a LB service.
// This should match the formatting of makeFirewallName() in pkg/cloudprovider/providers/gce/gce_loadbalancer.go
func MakeFirewallNameForLBService(name string) string {
	return fmt.Sprintf("k8s-fw-%s", name)
}

// WaitForFirewallRule waits for the specified firewall existence
func WaitForFirewallRule(ctx context.Context, gceCloud *gcecloud.Cloud, fwName string, exist bool, timeout time.Duration) (*compute.Firewall, error) {
	framework.Logf("Waiting up to %v for firewall %v exist=%v", timeout, fwName, exist)
	var fw *compute.Firewall
	var err error

	condition := func(ctx context.Context) (bool, error) {
		fw, err = gceCloud.GetFirewall(fwName)
		if err != nil && exist ||
			err == nil && !exist ||
			err != nil && !exist && !IsGoogleAPIHTTPErrorCode(err, http.StatusNotFound) {
			return false, nil
		}
		return true, nil
	}

	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, condition); err != nil {
		return nil, fmt.Errorf("error waiting for firewall %v exist=%v", fwName, exist)
	}
	return fw, nil
}

// VerifyFirewallRule verifies whether the result firewall is consistent with the expected firewall.
// When `portsSubset` is false, match given ports exactly. Otherwise, only check ports are included.
func VerifyFirewallRule(res, exp *compute.Firewall, network string, portsSubset bool) error {
	if res == nil || exp == nil {
		return fmt.Errorf("res and exp must not be nil")
	}
	if res.Name != exp.Name {
		return fmt.Errorf("incorrect name: %v, expected %v", res.Name, exp.Name)
	}

	actualPorts := PackProtocolsPortsFromFirewall(res.Allowed)
	expPorts := PackProtocolsPortsFromFirewall(exp.Allowed)
	if portsSubset {
		if err := isPortsSubset(expPorts, actualPorts); err != nil {
			return fmt.Errorf("incorrect allowed protocol ports: %w", err)
		}
	} else {
		if err := SameStringArray(actualPorts, expPorts, false); err != nil {
			return fmt.Errorf("incorrect allowed protocols ports: %w", err)
		}
	}

	if err := SameStringArray(res.SourceRanges, exp.SourceRanges, false); err != nil {
		return fmt.Errorf("incorrect source ranges %v, expected %v: %w", res.SourceRanges, exp.SourceRanges, err)
	}
	if err := SameStringArray(res.SourceTags, exp.SourceTags, false); err != nil {
		return fmt.Errorf("incorrect source tags %v, expected %v: %w", res.SourceTags, exp.SourceTags, err)
	}
	if err := SameStringArray(res.TargetTags, exp.TargetTags, false); err != nil {
		return fmt.Errorf("incorrect target tags %v, expected %v: %w", res.TargetTags, exp.TargetTags, err)
	}
	return nil
}

// ConstructFirewallForLBService returns the expected GCE firewall rule for a loadbalancer type service
func ConstructFirewallForLBService(svc *v1.Service, nodeTag string) *compute.Firewall {
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		framework.Failf("can not construct firewall rule for non-loadbalancer type service")
	}
	fw := compute.Firewall{}
	fw.Name = MakeFirewallNameForLBService(cloudprovider.DefaultLoadBalancerName(svc))
	fw.TargetTags = []string{nodeTag}
	if svc.Spec.LoadBalancerSourceRanges == nil {
		fw.SourceRanges = []string{"0.0.0.0/0"}
	} else {
		fw.SourceRanges = svc.Spec.LoadBalancerSourceRanges
	}
	for _, sp := range svc.Spec.Ports {
		fw.Allowed = append(fw.Allowed, &compute.FirewallAllowed{
			IPProtocol: strings.ToLower(string(sp.Protocol)),
			Ports:      []string{strconv.Itoa(int(sp.Port))},
		})
	}
	return &fw
}

// MakeHealthCheckFirewallNameForLBService returns the firewall name used by the GCE load
// balancers for performing health checks.
func MakeHealthCheckFirewallNameForLBService(clusterID, name string, isNodesHealthCheck bool) string {
	return gcecloud.MakeHealthCheckFirewallName(clusterID, name, isNodesHealthCheck)
}

// ConstructHealthCheckFirewallForLBService returns the expected GCE firewall rule for a loadbalancer type service
func ConstructHealthCheckFirewallForLBService(clusterID string, svc *v1.Service, nodeTag string, isNodesHealthCheck bool) *compute.Firewall {
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		framework.Failf("can not construct firewall rule for non-loadbalancer type service")
	}
	fw := compute.Firewall{}
	fw.Name = MakeHealthCheckFirewallNameForLBService(clusterID, cloudprovider.DefaultLoadBalancerName(svc), isNodesHealthCheck)
	fw.TargetTags = []string{nodeTag}
	fw.SourceRanges = gcecloud.L4LoadBalancerSrcRanges()
	healthCheckPort := gcecloud.GetNodesHealthCheckPort()
	if !isNodesHealthCheck {
		healthCheckPort = svc.Spec.HealthCheckNodePort
	}
	fw.Allowed = []*compute.FirewallAllowed{
		{
			IPProtocol: "tcp",
			Ports:      []string{fmt.Sprintf("%d", healthCheckPort)},
		},
	}
	return &fw
}

// PackProtocolsPortsFromFirewall packs protocols and ports in an unified way for verification.
func PackProtocolsPortsFromFirewall(alloweds []*compute.FirewallAllowed) []string {
	protocolPorts := []string{}
	for _, allowed := range alloweds {
		for _, port := range allowed.Ports {
			protocolPorts = append(protocolPorts, strings.ToLower(allowed.IPProtocol+"/"+port))
		}
	}
	return protocolPorts
}

type portRange struct {
	protocol string
	min, max int
}

func toPortRange(s string) (pr portRange, err error) {
	protoPorts := strings.Split(s, "/")
	// Set protocol
	pr.protocol = strings.ToUpper(protoPorts[0])

	if len(protoPorts) != 2 {
		return pr, fmt.Errorf("expected a single '/' in %q", s)
	}

	ports := strings.Split(protoPorts[1], "-")
	switch len(ports) {
	case 1:
		v, err := strconv.Atoi(ports[0])
		if err != nil {
			return pr, err
		}
		pr.min, pr.max = v, v
	case 2:
		start, err := strconv.Atoi(ports[0])
		if err != nil {
			return pr, err
		}
		end, err := strconv.Atoi(ports[1])
		if err != nil {
			return pr, err
		}
		pr.min, pr.max = start, end
	default:
		return pr, fmt.Errorf("unexpected range value %q", protoPorts[1])
	}

	return pr, nil
}

// isPortsSubset asserts that the "requiredPorts" are covered by the "coverage" ports.
// requiredPorts - must be single-port, examples: 'tcp/50', 'udp/80'.
// coverage - single or port-range values, example: 'tcp/50', 'udp/80-1000'.
// Returns true if every requiredPort exists in the list of coverage rules.
func isPortsSubset(requiredPorts, coverage []string) error {
	for _, reqPort := range requiredPorts {
		rRange, err := toPortRange(reqPort)
		if err != nil {
			return err
		}
		if rRange.min != rRange.max {
			return fmt.Errorf("requiring a range is not supported: %q", reqPort)
		}

		var covered bool
		for _, c := range coverage {
			cRange, err := toPortRange(c)
			if err != nil {
				return err
			}

			if rRange.protocol != cRange.protocol {
				continue
			}

			if rRange.min >= cRange.min && rRange.min <= cRange.max {
				covered = true
				break
			}
		}

		if !covered {
			return fmt.Errorf("%q is not covered by %v", reqPort, coverage)
		}
	}
	return nil
}

// SameStringArray verifies whether two string arrays have the same strings, return error if not.
// Order does not matter.
// When `include` is set to true, verifies whether result includes all elements from expected.
func SameStringArray(result, expected []string, include bool) error {
	res := sets.NewString(result...)
	exp := sets.NewString(expected...)
	if !include {
		diff := res.Difference(exp)
		if len(diff) != 0 {
			return fmt.Errorf("found differences: %v", diff)
		}
	} else {
		if !res.IsSuperset(exp) {
			return fmt.Errorf("some elements are missing: expected %v, got %v", expected, result)
		}
	}
	return nil
}
