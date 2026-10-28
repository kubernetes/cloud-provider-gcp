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

// Package integration runs the metis daemon and the CCM dynamic pod IP
// controllers in one process against fake Kubernetes and GCE APIs, so that the
// full control loop can be exercised end to end:
//
//	pod ADD/DEL -> metis daemon -> NodeNetworkConfig spec
//	  -> CCM spec controller -> (fake) GCE alias IP ranges
//	  -> CCM status controller -> NodeNetworkConfig status
//	  -> metis watcher/monitor -> SQLite
//
// Both sides share the same fake NodeNetworkConfig clientset, which plays the
// role of the API server.
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/go-logr/logr"
	computebeta "google.golang.org/api/compute/v0.beta"
	computepreview "google.golang.org/api/compute/v1.preview"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	gce "k8s.io/cloud-provider-gcp/providers/gce"

	pb "k8s.io/metis/api/adaptiveipam/v1"
	metispkg "k8s.io/metis/pkg"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/pkg/store"
)

const (
	nodeName = "test-node"
	zone     = "us-central1-a"
	project  = "test-project"
	network  = networkv1.DefaultPodNetworkName

	// initialPodCIDR is the primary alias range the node boots with. It reaches
	// the daemon through the CNI config (pb.IPConfig.InitialPodCidr), not through
	// CCM, which is what makes bootstrap interesting.
	initialPodCIDR = "10.0.0.0/28"

	// The daemon reserves the first two and the last IP of the first block of a
	// network, so a /28 initial block yields 13 allocatable IPs.
	initialUsableIPs = 13

	monitorInterval = 200 * time.Millisecond
	eventuallyTick  = 50 * time.Millisecond
	eventuallyMax   = 15 * time.Second
	// allocateTimeout must exceed the daemon's scale-up wait (9s) so a blocked
	// allocation surfaces as the daemon's own error rather than a client timeout.
	allocateTimeout = 20 * time.Second
)

var (
	providerID  = fmt.Sprintf("gce://%s/%s/%s", project, zone, nodeName)
	networkURL  = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/default", project)
	instanceKey = meta.ZonalKey(nodeName, zone)
)

type fixture struct {
	t   *testing.T
	ctx context.Context

	kubeClient    *k8sfake.Clientset
	nncClient     *nncfake.Clientset
	gce           *gce.Cloud
	mockInstances *gcloud.MockBetaInstances
	statusTrigger dynamicpodip.StatusTrigger

	// store is a second handle on the daemon's SQLite file, used for assertions.
	store *store.Store
	ipam  pb.AdaptiveIpamClient

	// aliasSeq hands out 10.100.<n>.0 for every "/28" the spec controller asks
	// the fake GCE for, so dynamically added ranges are predictable.
	aliasSeq       atomic.Int32
	fingerprintSeq atomic.Int64
}

type fixtureConfig struct {
	nodeProviderID       string
	backendType          dynamicpodip.BackendType
	daemonConfigModifier func(*daemon.Config)
	nncReactors          []customReactor
}

type customReactor struct {
	verb     string
	resource string
	reaction clientgotesting.ReactionFunc
}

type fixtureOption func(*fixtureConfig)

func withNodeProviderID(id string) fixtureOption {
	return func(cfg *fixtureConfig) {
		cfg.nodeProviderID = id
	}
}

func withBackend(backend dynamicpodip.BackendType) fixtureOption {
	return func(cfg *fixtureConfig) {
		cfg.backendType = backend
	}
}

func withDaemonConfig(modifier func(*daemon.Config)) fixtureOption {
	return func(cfg *fixtureConfig) {
		cfg.daemonConfigModifier = modifier
	}
}

func withPrependReactor(verb, resource string, reaction clientgotesting.ReactionFunc) fixtureOption {
	return func(cfg *fixtureConfig) {
		cfg.nncReactors = append(cfg.nncReactors, customReactor{
			verb:     verb,
			resource: resource,
			reaction: reaction,
		})
	}
}

// newFixture brings up fake GCE, the CCM controllers and the metis daemon, and
// returns once the daemon's gRPC socket accepts connections.
func newFixture(t *testing.T, opts ...fixtureOption) *fixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := &fixtureConfig{
		nodeProviderID: providerID,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	f := &fixture{t: t, ctx: ctx}

	// --- Fake Kubernetes API ---------------------------------------------------
	f.kubeClient = k8sfake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: "node-uid"},
		Spec:       corev1.NodeSpec{ProviderID: cfg.nodeProviderID},
	})
	f.nncClient = nncfake.NewSimpleClientset()
	emulateAPIServerSemantics(f.nncClient)
	for _, r := range cfg.nncReactors {
		f.nncClient.PrependReactor(r.verb, r.resource, r.reaction)
	}

	// --- Fake GCE ----------------------------------------------------------------
	values := gce.DefaultTestClusterValues()
	values.ProjectID = project
	values.ZoneName = zone
	values.NetworkURL = networkURL
	f.gce = gce.NewFakeGCECloud(values)

	mockInstances, ok := f.gce.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("fake GCE BetaInstances is %T, want *gcloud.MockBetaInstances", f.gce.Compute().BetaInstances())
	}
	f.mockInstances = mockInstances
	mockInstances.UpdateNetworkInterfaceHook = f.updateNetworkInterfaceHook

	if err := f.gce.Compute().Instances().Insert(ctx, instanceKey, &compute.Instance{
		Name: nodeName,
		Zone: zone,
		NetworkInterfaces: []*compute.NetworkInterface{{
			Name:          "nic0",
			Network:       networkURL,
			Subnetwork:    "default",
			AliasIpRanges: []*compute.AliasIpRange{{IpCidrRange: initialPodCIDR}},
		}},
	}); err != nil {
		t.Fatalf("failed to insert fake GCE instance: %v", err)
	}

	// --- CCM dynamic pod IP controllers -----------------------------------------
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

	// --- metis daemon ------------------------------------------------------------
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metis.sqlite")
	socketPath := filepath.Join(dir, "metis.sock")
	t.Setenv("NODE_NAME", nodeName)

	dConfig := daemon.Config{
		DBPath:                          dbPath,
		SocketPath:                      socketPath,
		MonitorInterval:                 monitorInterval,
		ReleaseCooldown:                 100 * time.Millisecond,
		DrainingExpiration:              200 * time.Millisecond,
		SustainedLowUtilizationDuration: 100 * time.Millisecond,
		LowUtilizationThreshold:         0.3,
	}
	if cfg.daemonConfigModifier != nil {
		cfg.daemonConfigModifier(&dConfig)
	}

	d := daemon.NewDaemon(dConfig)
	d.NNCClient = f.nncClient
	d.KubeClient = f.kubeClient
	go func() {
		if err := d.Run(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("daemon exited with error: %v", err)
		}
	}()
	f.eventually("daemon socket is ready", func() (bool, string) {
		_, err := os.Stat(socketPath)
		return err == nil, fmt.Sprintf("stat %s: %v", socketPath, err)
	})

	conn, err := metispkg.NewLocalGrpcConnection(socketPath)
	if err != nil {
		t.Fatalf("failed to connect to daemon socket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	f.ipam = pb.NewAdaptiveIpamClient(conn)

	s, err := store.NewStore(ctx, logr.Discard(), dbPath)
	if err != nil {
		t.Fatalf("failed to open store for assertions: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	f.store = s

	return f
}

// emulateAPIServerSemantics installs a reactor on the fake NodeNetworkConfig
// clientset that reproduces the three apiserver behaviours the controllers
// depend on and the fake tracker does not implement:
//
//   - Status subresource isolation: a status write must not clobber spec
//     fields written concurrently by the other side (the daemon patches spec
//     while CCM writes status).
//   - metadata.resourceVersion changes on every write. The metis watcher drops
//     updates whose resourceVersion did not change.
//   - metadata.generation increments when a non-status write changes the spec.
//     The CCM spec controller drops updates whose generation did not change.
//
// Writes go through the tracker rather than back through the clientset, because
// the clientset lock is held while a reactor runs.
func emulateAPIServerSemantics(c *nncfake.Clientset) {
	defaultReaction := clientgotesting.ObjectReaction(c.Tracker())
	c.PrependReactor("*", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		var name string
		switch a := action.(type) {
		case clientgotesting.CreateAction:
			obj, ok := a.GetObject().(*nncv1.NodeNetworkConfig)
			if !ok {
				return false, nil, nil
			}
			name = obj.Name
		case clientgotesting.UpdateAction:
			obj, ok := a.GetObject().(*nncv1.NodeNetworkConfig)
			if !ok {
				return false, nil, nil
			}
			name = obj.Name
		case clientgotesting.PatchAction:
			name = a.GetName()
		default:
			return false, nil, nil
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()

		var before *nncv1.NodeNetworkConfig
		if existing, err := c.Tracker().Get(gvr, ns, name); err == nil {
			before, _ = existing.(*nncv1.NodeNetworkConfig)
		}

		var after *nncv1.NodeNetworkConfig
		if update, ok := action.(clientgotesting.UpdateAction); ok && action.GetSubresource() == "status" && before != nil {
			// Status subresource: keep the stored spec, take the incoming status.
			after = update.GetObject().(*nncv1.NodeNetworkConfig).DeepCopy()
			after.Spec = *before.Spec.DeepCopy()
		} else {
			handled, obj, err := defaultReaction(action)
			if !handled || err != nil {
				return handled, obj, err
			}
			stored, ok := obj.(*nncv1.NodeNetworkConfig)
			if !ok {
				return true, obj, nil
			}
			after = stored.DeepCopy()
		}

		prevRV, prevGen := 0, int64(0)
		if before != nil {
			prevRV, _ = strconv.Atoi(before.ResourceVersion)
			prevGen = before.Generation
		}
		after.ResourceVersion = strconv.Itoa(prevRV + 1)
		if before == nil {
			after.Generation = 1
		} else if action.GetSubresource() == "" && !apiequality.Semantic.DeepEqual(before.Spec, after.Spec) {
			after.Generation = prevGen + 1
		} else {
			after.Generation = prevGen
		}

		if err := c.Tracker().Update(gvr, after, ns); err != nil {
			return true, nil, err
		}
		return true, after, nil
	})
}

// updateNetworkInterfaceHook emulates GCE's updateNetworkInterface on the mock
// instance store: it enforces the interface fingerprint and resolves "/28"
// placeholders into concrete ranges.
func (f *fixture) updateNetworkInterfaceHook(_ context.Context, key *meta.Key, ifaceName string, iface *computebeta.NetworkInterface, mock *gcloud.MockBetaInstances, _ ...gcloud.Option) error {
	mock.Lock.Lock()
	defer mock.Lock.Unlock()

	obj, ok := mock.Objects[*key]
	if !ok {
		return fmt.Errorf("instance %v not found in mock store", key)
	}
	instance := obj.ToBeta()
	for i, ni := range instance.NetworkInterfaces {
		if ni.Name != ifaceName {
			continue
		}
		if ni.Fingerprint != "" && iface.Fingerprint != ni.Fingerprint {
			return fmt.Errorf("stale fingerprint for interface %q: got %q, want %q", ifaceName, iface.Fingerprint, ni.Fingerprint)
		}
		instance.NetworkInterfaces[i].AliasIpRanges = f.resolveAliasRanges(iface.AliasIpRanges)
		instance.NetworkInterfaces[i].Fingerprint = fmt.Sprintf("fingerprint-%d", f.fingerprintSeq.Add(1))
		mock.Objects[*key] = &gcloud.MockInstancesObj{Obj: instance}
		return nil
	}
	return fmt.Errorf("network interface %q not found on mock instance %v", ifaceName, key)
}

func (f *fixture) resolveAliasRanges(ranges []*computebeta.AliasIpRange) []*computebeta.AliasIpRange {
	resolved := make([]*computebeta.AliasIpRange, 0, len(ranges))
	for _, r := range ranges {
		out := *r
		if len(out.IpCidrRange) > 0 && out.IpCidrRange[0] == '/' {
			out.IpCidrRange = fmt.Sprintf("10.100.%d.0%s", f.aliasSeq.Add(1)-1, out.IpCidrRange)
		}
		resolved = append(resolved, &out)
	}
	return resolved
}

// --- pod lifecycle -----------------------------------------------------------

func (f *fixture) allocatePod(id string) (ip, cidr string) {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, allocateTimeout)
	defer cancel()
	resp, err := f.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config: &pb.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    id,
			InitialPodCidr: initialPodCIDR,
		},
		PodName:      id,
		PodNamespace: "default",
	})
	if err != nil {
		f.t.Fatalf("AllocatePodIP(%s) failed: %v", id, err)
	}
	return resp.GetIpv4().GetIpAddress(), resp.GetIpv4().GetCidr()
}

// tryAllocatePod is allocatePod for callers that expect failure.
func (f *fixture) tryAllocatePod(id string) (cidr string, err error) {
	ctx, cancel := context.WithTimeout(f.ctx, allocateTimeout)
	defer cancel()
	resp, err := f.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config: &pb.IPConfig{InterfaceName: "eth0", ContainerId: id, InitialPodCidr: initialPodCIDR},
		PodName:    id,
	})
	if err != nil {
		return "", err
	}
	return resp.GetIpv4().GetCidr(), nil
}

func (f *fixture) deletePod(id string) {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, allocateTimeout)
	defer cancel()
	if _, err := f.ipam.DeallocatePodIP(ctx, &pb.DeallocatePodIPRequest{InterfaceName: "eth0", ContainerId: id, PodName: id}); err != nil {
		f.t.Fatalf("DeallocatePodIP(%s) failed: %v", id, err)
	}
}

// --- GCE ---------------------------------------------------------------------

// gceAliasRanges returns the alias ranges currently attached to nic0 in fake
// GCE.
func (f *fixture) gceAliasRanges() []string {
	f.t.Helper()
	f.mockInstances.Lock.Lock()
	defer f.mockInstances.Lock.Unlock()
	obj, ok := f.mockInstances.Objects[*instanceKey]
	if !ok {
		f.t.Fatalf("instance %v missing from fake GCE", instanceKey)
	}
	var ranges []string
	for _, r := range obj.ToBeta().NetworkInterfaces[0].AliasIpRanges {
		ranges = append(ranges, r.IpCidrRange)
	}
	return ranges
}

// gceANEs returns all active AliasNetworkEndpoint resources attached to this instance.
func (f *fixture) gceANEs() []*computepreview.AliasNetworkEndpoint {
	f.t.Helper()
	anes, err := f.gce.ListAliasNetworkEndpoints(f.ctx, providerID)
	if err != nil {
		f.t.Fatalf("failed to list ANEs: %v", err)
	}
	return anes
}

// gceCIDRs returns all active pod CIDRs attached to the instance across both
// VM alias IP ranges and AliasNetworkEndpoints.
func (f *fixture) gceCIDRs() []string {
	f.t.Helper()
	ranges := f.gceAliasRanges()
	for _, ane := range f.gceANEs() {
		for _, alias := range ane.Aliases {
			if alias.EffectiveIpAddress != "" {
				ranges = append(ranges, alias.EffectiveIpAddress+"/32")
			}
		}
	}
	return ranges
}

// updateNodeProviderID updates the Node's Spec.ProviderID in the fake kube client.
func (f *fixture) updateNodeProviderID(id string) {
	f.t.Helper()
	node, err := f.kubeClient.CoreV1().Nodes().Get(f.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		f.t.Fatalf("failed to get node: %v", err)
	}
	node = node.DeepCopy()
	node.Spec.ProviderID = id
	if _, err := f.kubeClient.CoreV1().Nodes().Update(f.ctx, node, metav1.UpdateOptions{}); err != nil {
		f.t.Fatalf("failed to update node providerID: %v", err)
	}
}

// addReleasableCIDR appends a CIDR to NodeNetworkConfig.spec.releasableCIDRs.
func (f *fixture) addReleasableCIDR(cidr string) {
	f.t.Helper()
	nnc := f.nnc().DeepCopy()
	nnc.Spec.ReleasableCIDRs = append(nnc.Spec.ReleasableCIDRs, nncv1.PodCIDR{
		Network: network,
		CIDR:    cidr,
	})
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Update(f.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		f.t.Fatalf("failed to add releasable CIDR %s: %v", cidr, err)
	}
}

// setAllocationsPods updates NodeNetworkConfig.spec.allocations for the default network.
func (f *fixture) setAllocationsPods(pods int32) {
	f.t.Helper()
	nnc := f.nnc().DeepCopy()
	found := false
	for i, a := range nnc.Spec.Allocations {
		if a.Network == network {
			nnc.Spec.Allocations[i].Pods = pods
			found = true
			break
		}
	}
	if !found {
		nnc.Spec.Allocations = append(nnc.Spec.Allocations, nncv1.Allocation{
			Network: network,
			Pods:    pods,
		})
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Update(f.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		f.t.Fatalf("failed to set allocations pods to %d: %v", pods, err)
	}
}

// nodeEvents returns the messages of the Events recorded against the node with
// the given reason. The CCM status controller's event broadcaster writes to the
// fake clientset asynchronously, so callers poll this.
func (f *fixture) nodeEvents(reason string) []string {
	f.t.Helper()
	list, err := f.kubeClient.CoreV1().Events("").List(f.ctx, metav1.ListOptions{})
	if err != nil {
		f.t.Fatalf("failed to list events: %v", err)
	}
	var messages []string
	for _, ev := range list.Items {
		if ev.Reason == reason && ev.InvolvedObject.Kind == "Node" && ev.InvolvedObject.Name == nodeName {
			messages = append(messages, ev.Message)
		}
	}
	return messages
}

// --- NodeNetworkConfig -------------------------------------------------------

func (f *fixture) nnc() *nncv1.NodeNetworkConfig {
	f.t.Helper()
	nnc, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(f.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		f.t.Fatalf("failed to get NodeNetworkConfig: %v", err)
	}
	return nnc
}

func statusCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var cidrs []string
	for _, pc := range nnc.Status.PodCIDRs {
		if pc.Network == network {
			cidrs = append(cidrs, pc.CIDR)
		}
	}
	return cidrs
}

func statusReady(nnc *nncv1.NodeNetworkConfig) bool {
	return apimeta.IsStatusConditionTrue(nnc.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
}

func specPods(nnc *nncv1.NodeNetworkConfig) int32 {
	for _, a := range nnc.Spec.Allocations {
		if a.Network == network {
			return a.Pods
		}
	}
	return 0
}

func releasableCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var cidrs []string
	for _, pc := range nnc.Spec.ReleasableCIDRs {
		cidrs = append(cidrs, pc.CIDR)
	}
	return cidrs
}

// --- SQLite ------------------------------------------------------------------

// blockState returns the state of a block in SQLite, or "" if the block is
// absent.
func (f *fixture) blockState(cidr string) store.CidrBlockState {
	f.t.Helper()
	var state string
	err := f.store.DB().QueryRowContext(f.ctx, "SELECT state FROM cidr_blocks WHERE cidr = ? AND network = ?", cidr, network).Scan(&state)
	if err != nil {
		return ""
	}
	return store.CidrBlockState(state)
}

func (f *fixture) blockExists(cidr string) bool {
	f.t.Helper()
	_, exists, err := f.store.GetCIDRBlock(f.ctx, cidr, network)
	if err != nil {
		f.t.Fatalf("GetCIDRBlock(%s) failed: %v", cidr, err)
	}
	return exists
}

func (f *fixture) usage() store.NetworkIPUsage {
	f.t.Helper()
	u, err := f.store.GetIPUsage(f.ctx, network, store.IPv4)
	if err != nil {
		f.t.Fatalf("GetIPUsage failed: %v", err)
	}
	return u
}

// --- helpers -----------------------------------------------------------------

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		if seen[s] == 0 {
			return false
		}
		seen[s]--
	}
	return true
}

// eventually polls cond until it reports true or eventuallyMax elapses. The
// second return value is included in the failure message to explain what the
// system looked like when the test gave up.
func (f *fixture) eventually(desc string, cond func() (bool, string)) {
	f.t.Helper()
	var last string
	err := wait.PollUntilContextTimeout(f.ctx, eventuallyTick, eventuallyMax, true, func(context.Context) (bool, error) {
		ok, detail := cond()
		last = detail
		return ok, nil
	})
	if err != nil {
		f.t.Fatalf("timed out waiting for %s; last observation: %s", desc, last)
	}
}

// consistently asserts that cond holds at every tick for the given duration.
func (f *fixture) consistently(desc string, d time.Duration, cond func() (bool, string)) {
	f.t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok, detail := cond(); !ok {
			f.t.Fatalf("%s violated: %s", desc, detail)
		}
		time.Sleep(eventuallyTick)
	}
}

// settledSpecPods returns spec.allocations.pods once it has stopped changing
// for several monitor ticks. The daemon may take more than one tick to reach
// its steady-state ask after capacity arrives (its baseline follows the IPs it
// holds, which land one block at a time), so a value read immediately after a
// scale-up is not yet the number later assertions should compare against.
func (f *fixture) settledSpecPods() int32 {
	f.t.Helper()
	quietPolls := int(3 * monitorInterval / eventuallyTick)
	var (
		last  int32 = -1
		quiet int
	)
	f.eventually("spec.allocations.pods to settle", func() (bool, string) {
		pods := specPods(f.nnc())
		if pods == last {
			quiet++
		} else {
			last, quiet = pods, 0
		}
		return quiet >= quietPolls, fmt.Sprintf("pods=%d quiet=%d/%d", pods, quiet, quietPolls)
	})
	return last
}
