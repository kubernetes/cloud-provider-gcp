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
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[cloud-provider-gcp-e2e] LoadBalancer", func() {
	f := framework.NewDefaultFramework("loadbalancer")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface
	BeforeEach(func() {
		cs = f.ClientSet
	})

	AfterEach(func() {
		// After each test
	})

	f.It("should be able to change the type and ports of a UDP service", f.WithSlow(), func(ctx context.Context) {
		loadBalancerLagTimeout := e2eservice.LoadBalancerLagTimeoutDefault
		loadBalancerCreateTimeout := e2eservice.GetServiceLoadBalancerCreationTimeout(ctx, cs)

		// This test is more monolithic than we'd like because LB turnup can be
		// very slow, so we lumped all the tests into one LB lifecycle.

		serviceName := "mutability-test"
		ns2 := f.Namespace.Name // LB1 in ns2 on TCP
		framework.Logf("namespace for TCP test: %s", ns2)

		By("creating a UDP service " + serviceName + " with type=ClusterIP in namespace " + ns2)
		udpJig := e2eservice.NewTestJig(cs, ns2, serviceName)
		udpService, err := udpJig.CreateUDPService(ctx, nil)
		framework.ExpectNoError(err)

		svcPort := int(udpService.Spec.Ports[0].Port)
		framework.Logf("service port UDP: %d", svcPort)

		By("creating a pod to be part of the UDP service " + serviceName)
		_, err = udpJig.Run(ctx, nil)
		framework.ExpectNoError(err)

		execPod := e2epod.CreateExecPodOrFail(ctx, cs, ns2, "execpod", nil)
		err = udpJig.CheckServiceReachability(ctx, udpService, execPod)
		framework.ExpectNoError(err)

		// Change the services to NodePort.

		By("changing the UDP service to type=NodePort")
		udpService, err = udpJig.UpdateService(ctx, func(s *v1.Service) {
			s.Spec.Type = v1.ServiceTypeNodePort
		})
		framework.ExpectNoError(err)
		udpNodePort := int(udpService.Spec.Ports[0].NodePort)
		framework.Logf("UDP node port: %d", udpNodePort)

		err = udpJig.CheckServiceReachability(ctx, udpService, execPod)
		framework.ExpectNoError(err)

		// Change the services to LoadBalancer.

		// Here we test that LoadBalancers can receive static IP addresses.  This isn't
		// necessary, but is an additional feature this monolithic test checks.
		requestedIP := ""

		staticIPName := ""
		By("creating a static load balancer IP")
		staticIPName = fmt.Sprintf("e2e-external-lb-test-%s", framework.RunID)
		gceCloud, err := GetGCECloud()
		framework.ExpectNoError(err, "failed to get GCE cloud provider")

		err = gceCloud.ReserveRegionAddress(&compute.Address{Name: staticIPName}, gceCloud.Region())
		defer func() {
			if staticIPName != "" {
				// Release GCE static IP - this is not kube-managed and will not be automatically released.
				if err := gceCloud.DeleteRegionAddress(staticIPName, gceCloud.Region()); err != nil {
					framework.Logf("failed to release static IP %s: %v", staticIPName, err)
				}
			}
		}()
		framework.ExpectNoError(err, "failed to create region address: %s", staticIPName)
		reservedAddr, err := gceCloud.GetRegionAddress(staticIPName, gceCloud.Region())
		framework.ExpectNoError(err, "failed to get region address: %s", staticIPName)

		requestedIP = reservedAddr.Address
		framework.Logf("Allocated static load balancer IP: %s", requestedIP)

		By("changing the UDP service to type=LoadBalancer")
		_, err = udpJig.UpdateService(ctx, func(s *v1.Service) {
			s.Spec.Type = v1.ServiceTypeLoadBalancer
		})
		framework.ExpectNoError(err)

		// Do this as early as possible, which overrides the `defer` above.
		// This is mostly out of fear of leaking the IP in a timeout case
		// (as of this writing we're not 100% sure where the leaks are
		// coming from, so this is first-aid rather than surgery).
		By("demoting the static IP to ephemeral")
		if staticIPName != "" {
			gceCloud, err := GetGCECloud()
			framework.ExpectNoError(err, "failed to get GCE cloud provider")
			// Deleting it after it is attached "demotes" it to an
			// ephemeral IP, which can be auto-released.
			if err := gceCloud.DeleteRegionAddress(staticIPName, gceCloud.Region()); err != nil {
				framework.Failf("failed to release static IP %s: %v", staticIPName, err)
			}
			staticIPName = ""
		}

		var udpIngressIP string
		By("waiting for the UDP service to have a load balancer")
		// 2nd one should be faster since they ran in parallel.
		udpService, err = udpJig.WaitForLoadBalancer(ctx, loadBalancerCreateTimeout)
		framework.ExpectNoError(err)
		if int(udpService.Spec.Ports[0].NodePort) != udpNodePort {
			framework.Failf("UDP Spec.Ports[0].NodePort changed (%d -> %d) when not expected", udpNodePort, udpService.Spec.Ports[0].NodePort)
		}
		udpIngressIP = e2eservice.GetIngressPoint(&udpService.Status.LoadBalancer.Ingress[0])
		framework.Logf("UDP load balancer: %s", udpIngressIP)

		err = udpJig.CheckServiceReachability(ctx, udpService, execPod)
		framework.ExpectNoError(err)

		By("hitting the UDP service's LoadBalancer")
		testReachableUDP(udpIngressIP, svcPort, loadBalancerLagTimeout)

		// Change the services' node ports.

		By("changing the UDP service's NodePort")
		udpService, err = udpJig.ChangeServiceNodePort(ctx, udpNodePort)
		framework.ExpectNoError(err)
		udpNodePortOld := udpNodePort
		udpNodePort = int(udpService.Spec.Ports[0].NodePort)
		if udpNodePort == udpNodePortOld {
			framework.Failf("UDP Spec.Ports[0].NodePort (%d) did not change", udpNodePort)
		}
		if e2eservice.GetIngressPoint(&udpService.Status.LoadBalancer.Ingress[0]) != udpIngressIP {
			framework.Failf("UDP Status.LoadBalancer.Ingress changed (%s -> %s) when not expected", udpIngressIP, e2eservice.GetIngressPoint(&udpService.Status.LoadBalancer.Ingress[0]))
		}
		framework.Logf("UDP node port: %d", udpNodePort)

		err = udpJig.CheckServiceReachability(ctx, udpService, execPod)
		framework.ExpectNoError(err)

		By("hitting the UDP service's LoadBalancer")
		testReachableUDP(udpIngressIP, svcPort, loadBalancerLagTimeout)

		// Change the services' main ports.

		By("changing the UDP service's port")
		udpService, err = udpJig.UpdateService(ctx, func(s *v1.Service) {
			s.Spec.Ports[0].Port++
		})
		framework.ExpectNoError(err)
		svcPortOld := svcPort
		svcPort = int(udpService.Spec.Ports[0].Port)
		if svcPort == svcPortOld {
			framework.Failf("UDP Spec.Ports[0].Port (%d) did not change", svcPort)
		}
		if int(udpService.Spec.Ports[0].NodePort) != udpNodePort {
			framework.Failf("UDP Spec.Ports[0].NodePort (%d) changed", udpService.Spec.Ports[0].NodePort)
		}
		if e2eservice.GetIngressPoint(&udpService.Status.LoadBalancer.Ingress[0]) != udpIngressIP {
			framework.Failf("UDP Status.LoadBalancer.Ingress changed (%s -> %s) when not expected", udpIngressIP, e2eservice.GetIngressPoint(&udpService.Status.LoadBalancer.Ingress[0]))
		}

		framework.Logf("service port UDP: %d", svcPort)

		By("hitting the UDP service's NodePort")
		err = udpJig.CheckServiceReachability(ctx, udpService, execPod)
		framework.ExpectNoError(err)

		By("hitting the UDP service's LoadBalancer")
		testReachableUDP(udpIngressIP, svcPort, loadBalancerCreateTimeout)

		By("Scaling the pods to 0")
		err = udpJig.Scale(0)
		framework.ExpectNoError(err)

		By("looking for ICMP REJECT on the UDP service's LoadBalancer")
		testRejectedUDP(udpIngressIP, svcPort, loadBalancerCreateTimeout)

		By("Scaling the pods to 1")
		err = udpJig.Scale(1)
		framework.ExpectNoError(err)

		By("hitting the UDP service's NodePort")
		err = udpJig.CheckServiceReachability(ctx, udpService, execPod)
		framework.ExpectNoError(err)

		By("hitting the UDP service's LoadBalancer")
		testReachableUDP(udpIngressIP, svcPort, loadBalancerCreateTimeout)

		// Change the services back to ClusterIP.

		By("changing UDP service back to type=ClusterIP")
		udpReadback, err := udpJig.UpdateService(ctx, func(s *v1.Service) {
			s.Spec.Type = v1.ServiceTypeClusterIP
		})
		framework.ExpectNoError(err)
		if udpReadback.Spec.Ports[0].NodePort != 0 {
			framework.Fail("UDP Spec.Ports[0].NodePort was not cleared")
		}
		// Wait for the load balancer to be destroyed asynchronously
		_, err = udpJig.WaitForLoadBalancerDestroy(ctx, udpIngressIP, svcPort, loadBalancerCreateTimeout)
		framework.ExpectNoError(err)
		if udpReadback.Spec.Ports[0].NodePort != 0 {
			framework.Fail("UDP Spec.Ports[0].NodePort was not cleared")
		}
		// Wait for the load balancer to be destroyed asynchronously
		_, err = udpJig.WaitForLoadBalancerDestroy(ctx, udpIngressIP, svcPort, loadBalancerCreateTimeout)
		framework.ExpectNoError(err)

		By("checking the UDP LoadBalancer is closed")
		testNotReachableUDP(udpIngressIP, svcPort, loadBalancerLagTimeout)
	})
})

// Helper functions for loadbalancer tests.

// HTTPPokeParams is a struct for HTTP poke parameters.
type HTTPPokeParams struct {
	Timeout        time.Duration // default = 10 secs
	ExpectCode     int           // default = 200
	BodyContains   string
	RetriableCodes []int
	EnableHTTPS    bool
}

// HTTPPokeResult is a struct for HTTP poke result.
type HTTPPokeResult struct {
	Status HTTPPokeStatus
	Code   int    // HTTP code: 0 if the connection was not made
	Error  error  // if there was any error
	Body   []byte // if code != 0
}

// HTTPPokeStatus is string for representing HTTP poke status.
type HTTPPokeStatus string

const (
	// HTTPSuccess is HTTP poke status which is success.
	HTTPSuccess HTTPPokeStatus = "Success"
	// HTTPError is HTTP poke status which is error.
	HTTPError HTTPPokeStatus = "UnknownError"
	// HTTPTimeout is HTTP poke status which is timeout.
	HTTPTimeout HTTPPokeStatus = "TimedOut"
	// HTTPRefused is HTTP poke status which is connection refused.
	HTTPRefused HTTPPokeStatus = "ConnectionRefused"
	// HTTPRetryCode is HTTP poke status which is retry code.
	HTTPRetryCode HTTPPokeStatus = "RetryCode"
	// HTTPWrongCode is HTTP poke status which is wrong code.
	HTTPWrongCode HTTPPokeStatus = "WrongCode"
	// HTTPBadResponse is HTTP poke status which is bad response.
	HTTPBadResponse HTTPPokeStatus = "BadResponse"
	// Any time we add new errors, we should audit all callers of this.
)

// PokeHTTP tries to connect to a host on a port for a given URL path.  Callers
// can specify additional success parameters, if desired.
//
// The result status will be characterized as precisely as possible, given the
// known users of this.
//
// The result code will be zero in case of any failure to connect, or non-zero
// if the HTTP transaction completed (even if the other test params make this a
// failure).
//
// The result error will be populated for any status other than Success.
//
// The result body will be populated if the HTTP transaction was completed, even
// if the other test params make this a failure).
func PokeHTTP(host string, port int, path string, params *HTTPPokeParams) HTTPPokeResult {
	// Set default params.
	if params == nil {
		params = &HTTPPokeParams{}
	}

	hostPort := net.JoinHostPort(host, strconv.Itoa(port))
	var url string
	if params.EnableHTTPS {
		url = fmt.Sprintf("https://%s%s", hostPort, path)
	} else {
		url = fmt.Sprintf("http://%s%s", hostPort, path)
	}

	ret := HTTPPokeResult{}

	// Sanity check inputs, because it has happened.  These are the only things
	// that should hard fail the test - they are basically ASSERT()s.
	if host == "" {
		framework.Failf("Got empty host for HTTP poke (%s)", url)
		return ret
	}
	if port == 0 {
		framework.Failf("Got port==0 for HTTP poke (%s)", url)
		return ret
	}

	if params.ExpectCode == 0 {
		params.ExpectCode = http.StatusOK
	}

	if params.Timeout == 0 {
		params.Timeout = 10 * time.Second
	}

	framework.Logf("Poking %q", url)

	resp, err := httpGetNoConnectionPoolTimeout(url, params.Timeout)
	if err != nil {
		ret.Error = err
		neterr, ok := err.(net.Error)
		if ok && neterr.Timeout() {
			ret.Status = HTTPTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			ret.Status = HTTPRefused
		} else {
			ret.Status = HTTPError
		}
		framework.Logf("Poke(%q): %v", url, err)
		return ret
	}

	ret.Code = resp.StatusCode

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ret.Status = HTTPError
		ret.Error = fmt.Errorf("error reading HTTP body: %w", err)
		framework.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}
	ret.Body = make([]byte, len(body))
	copy(ret.Body, body)

	if resp.StatusCode != params.ExpectCode {
		for _, code := range params.RetriableCodes {
			if resp.StatusCode == code {
				ret.Error = fmt.Errorf("retriable status code: %d", resp.StatusCode)
				ret.Status = HTTPRetryCode
				framework.Logf("Poke(%q): %v", url, ret.Error)
				return ret
			}
		}
		ret.Status = HTTPWrongCode
		ret.Error = fmt.Errorf("bad status code: %d", resp.StatusCode)
		framework.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}

	if params.BodyContains != "" && !strings.Contains(string(body), params.BodyContains) {
		ret.Status = HTTPBadResponse
		ret.Error = fmt.Errorf("response does not contain expected substring: %q", string(body))
		framework.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}

	ret.Status = HTTPSuccess
	framework.Logf("Poke(%q): success", url)
	return ret
}

// Does an HTTP GET, but does not reuse TCP connections
// This masks problems where the iptables rule has changed, but we don't see it
func httpGetNoConnectionPoolTimeout(url string, timeout time.Duration) (*http.Response, error) {
	tr := utilnet.SetTransportDefaults(&http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
	})
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	return client.Get(url)
}

// testReachableUDP tests that the given host serves UDP on the given port.
func testReachableUDP(host string, port int, timeout time.Duration) {
	pollfn := func() (bool, error) {
		result := pokeUDP(host, port, "echo hello", &UDPPokeParams{
			Timeout:  3 * time.Second,
			Response: "hello",
		})
		if result.Status == UDPSuccess {
			return true, nil
		}
		return false, nil // caller can retry
	}

	if err := wait.PollImmediate(framework.Poll, timeout, pollfn); err != nil {
		framework.Failf("Could not reach UDP service through %v:%v after %v: %v", host, port, timeout, err)
	}
}

// testNotReachableUDP tests that the given host doesn't serve UDP on the given port.
func testNotReachableUDP(host string, port int, timeout time.Duration) {
	pollfn := func() (bool, error) {
		result := pokeUDP(host, port, "echo hello", &UDPPokeParams{Timeout: 3 * time.Second})
		if result.Status != UDPSuccess && result.Status != UDPError {
			return true, nil
		}
		return false, nil // caller can retry
	}
	if err := wait.PollImmediate(framework.Poll, timeout, pollfn); err != nil {
		framework.Failf("UDP service %v:%v reachable after %v: %v", host, port, timeout, err)
	}
}

// testRejectedUDP tests that the given host rejects a UDP request on the given port.
func testRejectedUDP(host string, port int, timeout time.Duration) {
	pollfn := func() (bool, error) {
		result := pokeUDP(host, port, "echo hello", &UDPPokeParams{Timeout: 3 * time.Second})
		if result.Status == UDPRefused {
			return true, nil
		}
		return false, nil // caller can retry
	}
	if err := wait.PollImmediate(framework.Poll, timeout, pollfn); err != nil {
		framework.Failf("UDP service %v:%v not rejected: %v", host, port, err)
	}
}

// UDPPokeParams is a struct for UDP poke parameters.
type UDPPokeParams struct {
	Timeout  time.Duration
	Response string
}

// UDPPokeResult is a struct for UDP poke result.
type UDPPokeResult struct {
	Status   UDPPokeStatus
	Error    error  // if there was any error
	Response []byte // if code != 0
}

// UDPPokeStatus is string for representing UDP poke status.
type UDPPokeStatus string

const (
	// UDPSuccess is UDP poke status which is success.
	UDPSuccess UDPPokeStatus = "Success"
	// UDPError is UDP poke status which is error.
	UDPError UDPPokeStatus = "UnknownError"
	// UDPTimeout is UDP poke status which is timeout.
	UDPTimeout UDPPokeStatus = "TimedOut"
	// UDPRefused is UDP poke status which is connection refused.
	UDPRefused UDPPokeStatus = "ConnectionRefused"
	// UDPBadResponse is UDP poke status which is bad response.
	UDPBadResponse UDPPokeStatus = "BadResponse"
	// Any time we add new errors, we should audit all callers of this.
)

// pokeUDP tries to connect to a host on a port and send the given request. Callers
// can specify additional success parameters, if desired.
//
// The result status will be characterized as precisely as possible, given the
// known users of this.
//
// The result error will be populated for any status other than Success.
//
// The result response will be populated if the UDP transaction was completed, even
// if the other test params make this a failure).
func pokeUDP(host string, port int, request string, params *UDPPokeParams) UDPPokeResult {
	hostPort := net.JoinHostPort(host, strconv.Itoa(port))
	url := fmt.Sprintf("udp://%s", hostPort)

	ret := UDPPokeResult{}

	// Sanity check inputs, because it has happened.  These are the only things
	// that should hard fail the test - they are basically ASSERT()s.
	if host == "" {
		framework.Failf("Got empty host for UDP poke (%s)", url)
		return ret
	}
	if port == 0 {
		framework.Failf("Got port==0 for UDP poke (%s)", url)
		return ret
	}

	// Set default params.
	if params == nil {
		params = &UDPPokeParams{}
	}

	framework.Logf("Poking %v", url)

	con, err := net.Dial("udp", hostPort)
	if err != nil {
		ret.Status = UDPError
		ret.Error = err
		framework.Logf("Poke(%q): %v", url, err)
		return ret
	}

	_, err = con.Write([]byte(fmt.Sprintf("%s\n", request)))
	if err != nil {
		ret.Error = err
		neterr, ok := err.(net.Error)
		if ok && neterr.Timeout() {
			ret.Status = UDPTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			ret.Status = UDPRefused
		} else {
			ret.Status = UDPError
		}
		framework.Logf("Poke(%q): %v", url, err)
		return ret
	}

	if params.Timeout != 0 {
		err = con.SetDeadline(time.Now().Add(params.Timeout))
		if err != nil {
			ret.Status = UDPError
			ret.Error = err
			framework.Logf("Poke(%q): %v", url, err)
			return ret
		}
	}

	bufsize := len(params.Response) + 1
	if bufsize == 0 {
		bufsize = 4096
	}
	var buf = make([]byte, bufsize)
	n, err := con.Read(buf)
	if err != nil {
		ret.Error = err
		neterr, ok := err.(net.Error)
		if ok && neterr.Timeout() {
			ret.Status = UDPTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			ret.Status = UDPRefused
		} else {
			ret.Status = UDPError
		}
		framework.Logf("Poke(%q): %v", url, err)
		return ret
	}
	ret.Response = buf[0:n]

	if params.Response != "" && string(ret.Response) != params.Response {
		ret.Status = UDPBadResponse
		ret.Error = fmt.Errorf("response does not match expected string: %q", string(ret.Response))
		framework.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}

	ret.Status = UDPSuccess
	framework.Logf("Poke(%q): success", url)
	return ret
}
