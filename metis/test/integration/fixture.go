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

// Subsystem: Test harness orchestrator and cluster-level test environment.
//
// Scope & Architecture:
// This file owns cluster-wide infrastructure initialization, fake clientset
// lifecycle, asynchronous polling primitives (Eventually, Consistently), and
// common assertion utilities. Single-node scenarios interact with this file
// directly via the embedded Node interface.
//
// Package integration tests the Metis CNI IPAM plugin, the Metis daemon, and
// the CCM dynamic pod IP controllers end-to-end against fake GCE and fake
// Kubernetes apiservers.
//
// The harness exercises the full end-to-end loop:
//
//	pod ADD/DEL via Metis CNI IPAM plugin (CmdAdd/CmdDel)
//	  -> Metis daemon (gRPC / direct DB fallback)
//	  -> NodeNetworkConfig spec
//	  -> CCM spec controller -> (fake) GCE alias IP ranges / ANEs
//	  -> CCM status controller -> NodeNetworkConfig status
//	  -> Metis watcher/monitor -> SQLite
//
// Rather than calling the gRPC daemon directly, all pod lifecycle operations
// (allocatePod, deletePod) run through the real Metis CNI IPAM plugin
// in-process, exercising full CNI config parsing, stdin/args serialization,
// IP selection, routes, and deallocation workflows.
//
// Both the Metis daemon and CCM controllers share the same fake
// NodeNetworkConfig clientset, which plays the role of the Kubernetes API
// server.
package integration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/metis/pkg/daemon"
)

// Default test environment topology constants.
const (
	// firstNodeName is the default node name provisioned automatically during
	// fixture initialization.
	firstNodeName  = "test-node"
	defaultZone    = "us-central1-a"
	defaultProject = "test-project"
	defaultNetwork = networkv1.DefaultPodNetworkName

	// InitialPodCIDR is the primary alias range the node boots with. It reaches
	// the daemon through the CNI config (pb.IPConfig.InitialPodCidr), not through
	// CCM, which is what makes bootstrap interesting.
	InitialPodCIDR = "10.0.0.0/28"

	// InitialPodCIDRv6 is the default initial IPv6 prefix used for DualStack and
	// IPv6Only modes.
	InitialPodCIDRv6 = "2001:db8:1::/112"

	// InitialUsableIPs: the daemon reserves the first two and the last IP of the
	// first block of a network, so a /28 initial block yields 13 allocatable
	// IPs.
	InitialUsableIPs = 13

	// MonitorInterval is the polling interval for the Metis daemon monitor.
	MonitorInterval = 200 * time.Millisecond
	// EventuallyTick is the polling interval for Eventually and Consistently.
	EventuallyTick = 50 * time.Millisecond
	// EventuallyMax is the default timeout for Eventually assertions.
	EventuallyMax = 15 * time.Second
	// AllocateTimeout must exceed the daemon's scale-up wait (9s) so a blocked
	// allocation surfaces as the daemon's own error rather than a client
	// timeout.
	AllocateTimeout = 20 * time.Second
)

// NodeProviderID returns the canonical GCE provider ID for any node in the
// test cluster.
func NodeProviderID(nodeName string) string {
	return fmt.Sprintf("gce://%s/%s/%s", defaultProject, defaultZone, nodeName)
}

// NodeInstanceKey returns the GCE compute metadata key for any node in the
// test cluster.
func NodeInstanceKey(nodeName string) *meta.Key {
	return meta.ZonalKey(nodeName, defaultZone)
}

// GCENetworkURL returns the fully-qualified GCE resource URL for any VPC
// network in the test project.
func GCENetworkURL(networkName string) string {
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", defaultProject, networkName)
}

// StackType defines the IP address family configuration of the cluster and
// node.
type StackType string

// Supported StackType configurations.
const (
	// StackTypeIPv4 allocates and manages only IPv4 addresses.
	StackTypeIPv4 StackType = "IPv4"
	// StackTypeDualStack allocates and manages both IPv4 and IPv6 addresses.
	StackTypeDualStack StackType = "DualStack"
	// StackTypeIPv6Only allocates and manages only IPv6 addresses.
	StackTypeIPv6Only StackType = "IPv6Only"
)

// Fixture orchestrates an integration test environment, coupling simulated
// K8s and GCE backends with live CCM controllers and running Metis daemons.
// Fixture embeds the Node interface for the cluster's first node, promoting all
// node-scoped operations (AllocatePod, NNC, GCEAliasRanges, Usage, etc.) directly
// onto the fixture.
type Fixture struct {
	Node

	t   *testing.T
	ctx context.Context
	cfg FixtureConfig

	kubeClient    *k8sfake.Clientset
	nncClient     *nncfake.Clientset
	gce           *gce.Cloud
	mockInstances *gcloud.MockBetaInstances
	statusTrigger dynamicpodip.StatusTrigger

	// aliasSeq hands out 10.100.<n>.0 for every "/28" the spec controller asks
	// the fake GCE for, so dynamically added ranges are predictable.
	aliasSeq       atomic.Int32
	fingerprintSeq atomic.Int64

	nodes map[string]Node
}

// Ctx returns the test fixture's execution context.
func (f *Fixture) Ctx() context.Context { return f.ctx }

// KubeClient returns the fake Kubernetes clientset.
func (f *Fixture) KubeClient() *k8sfake.Clientset { return f.kubeClient }

// NNCClient returns the fake NodeNetworkConfig clientset.
func (f *Fixture) NNCClient() *nncfake.Clientset { return f.nncClient }

// GCE returns the fake GCE Cloud instance.
func (f *Fixture) GCE() *gce.Cloud { return f.gce }

// FixtureConfig stores configuration options for initializing a test fixture.
//
// Note on future Shared VPC (XPN) extension:
// In standard single-project testing, GCE VM instances and the VPC network reside
// in the same project (defaultProject). When cross-project / Shared VPC scenarios
// are introduced, this config can be extended with a networkProjectID field and
// WithSharedVPC(hostProject string) option, allowing instances to run in a service
// project while network resources resolve against the host project.
type FixtureConfig struct {
	nodeProviderID       string
	backendType          dynamicpodip.BackendType
	stackType            StackType
	initialIPv4CIDR      string
	initialIPv6CIDR      string
	daemonConfigModifier func(*daemon.Config)
	nncReactors          []customReactor
}

// FixtureOption configures a FixtureConfig during test fixture initialization.
type FixtureOption func(*FixtureConfig)

// WithStackType sets the cluster stack configuration (IPv4, DualStack, or
// IPv6Only).
func WithStackType(st StackType) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.stackType = st
	}
}

// WithInitialAliasIP configures a custom initial IPv4 alias IP range on the
// primary interface.
func WithInitialAliasIP(cidr string) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.initialIPv4CIDR = cidr
	}
}

// WithInitialPodCIDRv6 configures a custom initial IPv6 CIDR prefix for
// dual-stack or IPv6-only modes.
func WithInitialPodCIDRv6(cidr string) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.initialIPv6CIDR = cidr
	}
}

// WithNodeProviderID sets the ProviderID on the simulated Node object.
// Passing an empty string simulates an uninitialized or NotReady node.
func WithNodeProviderID(id string) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.nodeProviderID = id
	}
}

// WithBackend configures whether the dynamic pod IP controllers run in
// Alias IP or Alias Network Endpoint (ANE) mode.
func WithBackend(backend dynamicpodip.BackendType) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.backendType = backend
	}
}

// WithDaemonConfig allows caller tests to customize Metis daemon configuration
// parameters (such as release cooldowns, monitor intervals, and thresholds).
func WithDaemonConfig(modifier func(*daemon.Config)) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.daemonConfigModifier = modifier
	}
}

// WithPrependReactor registers a custom client-go reactor on the fake NNC
// clientset, allowing tests to inject specific API errors (such as 409
// Conflict) or intercept actions.
func WithPrependReactor(verb, resource string, reaction clientgotesting.ReactionFunc) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.nncReactors = append(cfg.nncReactors, customReactor{
			verb:     verb,
			resource: resource,
			reaction: reaction,
		})
	}
}

// NewFixture constructs and boots an isolated integration test environment
// containing a simulated K8s cluster with fake core and NNC clientsets, a
// simulated GCE compute environment with mock instance network interfaces,
// live CCM NodeNetworkConfig spec and status controllers, an in-process Metis
// CNI IPAM plugin instance wired to standard CNI network configurations, and a
// live Metis daemon backed by a real SQLite store and gRPC server. All
// background routines, sockets, and temporary files are automatically cleaned
// up when the test finishes.
func NewFixture(t *testing.T, opts ...FixtureOption) *Fixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := FixtureConfig{
		nodeProviderID:  NodeProviderID(firstNodeName),
		stackType:       StackTypeIPv4,
		initialIPv4CIDR: InitialPodCIDR,
		initialIPv6CIDR: InitialPodCIDRv6,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	f := &Fixture{
		t:          t,
		ctx:        ctx,
		cfg:        cfg,
		kubeClient: k8sfake.NewSimpleClientset(),
		nncClient:  nncfake.NewSimpleClientset(),
		nodes:      make(map[string]Node),
	}

	emulateAPIServerSemantics(f.nncClient)
	for _, r := range cfg.nncReactors {
		f.nncClient.PrependReactor(r.verb, r.resource, r.reaction)
	}

	// --- First Node in Kubernetes ---------------------------------------------
	if _, err := f.kubeClient.CoreV1().Nodes().Create(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: firstNodeName},
		Spec:       corev1.NodeSpec{ProviderID: cfg.nodeProviderID},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create fake Node: %v", err)
	}

	// --- Fake GCE -------------------------------------------------------------
	values := gce.DefaultTestClusterValues()
	values.ProjectID = defaultProject
	values.ZoneName = defaultZone
	values.NetworkURL = GCENetworkURL("default")
	f.gce = gce.NewFakeGCECloud(values)

	mockInstances, ok := f.gce.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("fake GCE BetaInstances is %T, want *gcloud.MockBetaInstances", f.gce.Compute().BetaInstances())
	}
	f.mockInstances = mockInstances
	mockInstances.UpdateNetworkInterfaceHook = f.updateNetworkInterfaceHook

	var initialAliasRanges []*compute.AliasIpRange
	if cfg.stackType != StackTypeIPv6Only && cfg.initialIPv4CIDR != "" {
		initialAliasRanges = []*compute.AliasIpRange{{IpCidrRange: cfg.initialIPv4CIDR}}
	}

	if err := f.gce.Compute().Instances().Insert(ctx, NodeInstanceKey(firstNodeName), &compute.Instance{
		Name: firstNodeName,
		Zone: defaultZone,
		NetworkInterfaces: []*compute.NetworkInterface{{
			Name:          "nic0",
			Network:       GCENetworkURL("default"),
			Subnetwork:    "default",
			AliasIpRanges: initialAliasRanges,
		}},
	}); err != nil {
		t.Fatalf("failed to insert fake GCE instance: %v", err)
	}

	// --- CCM dynamic pod IP controllers ---------------------------------------
	nodeInformerFactory := informers.NewSharedInformerFactory(f.kubeClient, 0)
	nodeInformer := nodeInformerFactory.Core().V1().Nodes()
	nodeInformer.Informer() // register before Start
	nodeInformerFactory.Start(ctx.Done())
	nodeInformerFactory.WaitForCacheSync(ctx.Done())

	statusTrigger, _, started, err := dynamicpodip.StartControllers(ctx, dynamicpodip.Options{
		EnableDynamicPodIPController: true,
		PodIPBackend:                 cfg.backendType,
		// A static candidate range keeps the spec controller off the Container API.
		MultiSecondaryRanges: []string{"pods-range=ACTIVE"},
	}, f.kubeClient, f.nncClient, nodeInformer, f.gce)
	if err != nil || !started {
		t.Fatalf("failed to start dynamic pod IP controllers: started=%v err=%v", started, err)
	}
	f.statusTrigger = statusTrigger

	// --- First Node & Daemon --------------------------------------------------
	f.Node = f.bootNode(
		firstNodeName,
		cfg.initialIPv4CIDR,
		cfg.initialIPv6CIDR,
		cfg.daemonConfigModifier,
	)

	return f
}

// DefaultNetworkName returns the logical name of the default pod network.
func (f *Fixture) DefaultNetworkName() string {
	return defaultNetwork
}

// Eventually polls cond until all assertions pass or EventuallyMax elapses.
// If the deadline expires, the test fails with an indented list of the last
// observed unmet conditions.
func (f *Fixture) Eventually(desc string, cond func(a *Assert)) {
	f.t.Helper()
	var lastDetail string
	err := wait.PollUntilContextTimeout(f.ctx, EventuallyTick, EventuallyMax, true, func(ctx context.Context) (bool, error) {
		a := &Assert{}
		cond(a)
		if len(a.unmet) > 0 {
			lastDetail = formatUnmet(a.unmet)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		f.t.Fatalf("timed out waiting for %s; last observation: %s", desc, lastDetail)
	}
}

// Consistently asserts that all assertions in cond remain passing for the full
// duration d, sampled every EventuallyTick. Useful for verifying that a quiescent
// state does not leak.
func (f *Fixture) Consistently(desc string, d time.Duration, cond func(a *Assert)) {
	f.t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		a := &Assert{}
		cond(a)
		if len(a.unmet) > 0 {
			f.t.Fatalf("consistently failed for %s: %s", desc, formatUnmet(a.unmet))
		}
		select {
		case <-f.ctx.Done():
			f.t.Fatalf("context cancelled during consistently(%s)", desc)
		case <-time.After(EventuallyTick):
		}
	}
}

// Assert collects condition failures during an Eventually or Consistently tick.
type Assert struct {
	unmet []string
}

// Check records an unmet condition if ok is false. It returns ok so it can be
// used as a guard clause (e.g. if !a.Check(obj != nil, "obj is nil") { return }).
func (a *Assert) Check(ok bool, format string, args ...any) bool {
	if !ok {
		a.unmet = append(a.unmet, fmt.Sprintf(format, args...))
	}
	return ok
}

// True asserts that condition is true.
func (a *Assert) True(desc string, condition bool) bool {
	return a.Check(condition, "%s is false", desc)
}

// False asserts that condition is false.
func (a *Assert) False(desc string, condition bool) bool {
	return a.Check(!condition, "%s is true", desc)
}

// Equal asserts that got equals want.
func (a *Assert) Equal(desc string, got, want any) bool {
	return a.Check(got == want, "%s: got %v, want %v", desc, got, want)
}

// EqualSets asserts that got and want contain the same elements regardless of order.
func (a *Assert) EqualSets(desc string, got, want []string) bool {
	return a.Check(EqualSets(got, want), "%s: got %v, want %v", desc, got, want)
}

// Fail unconditionally records a failure message.
func (a *Assert) Fail(format string, args ...any) {
	a.unmet = append(a.unmet, fmt.Sprintf(format, args...))
}

// Unmet returns the list of recorded unmet conditions.
func (a *Assert) Unmet() []string {
	return a.unmet
}

// FormatUnmet formats a list of unmet condition messages as an indented bulleted
// list. If unmet is empty, it returns (true, "all conditions satisfied").
func FormatUnmet(unmet []string) (bool, string) {
	if len(unmet) == 0 {
		return true, "all conditions satisfied"
	}
	return false, formatUnmet(unmet)
}

func formatUnmet(unmet []string) string {
	if len(unmet) == 0 {
		return "all conditions satisfied"
	}
	return "\n  - " + strings.Join(unmet, "\n  - ")
}

// EqualSets checks if two string slices contain the same elements, regardless
// of order.
func EqualSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := slices.Clone(a)
	bCopy := slices.Clone(b)
	slices.Sort(aCopy)
	slices.Sort(bCopy)
	return slices.Equal(aCopy, bCopy)
}
