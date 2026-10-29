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

// Package integration tests the Metis CNI IPAM plugin, the Metis daemon, and
// the CCM dynamic pod IP controllers end-to-end against fake GCE and fake Kubernetes apiservers.
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
// (allocatePod, deletePod) run through the real Metis CNI IPAM plugin in-process,
// exercising full CNI config parsing, stdin/args serialization, IP selection,
// routes, and deallocation workflows.
//
// Both the Metis daemon and CCM controllers share the same fake NodeNetworkConfig
// clientset, which plays the role of the Kubernetes API server.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/go-logr/logr"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	pb "k8s.io/metis/api/adaptiveipam/v1"
	metispkg "k8s.io/metis/pkg"
	"k8s.io/metis/pkg/cni"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/pkg/store"
)

const (
	NodeName = "test-node"
	Zone     = "us-central1-a"
	Project  = "test-project"
	Network  = networkv1.DefaultPodNetworkName

	// InitialPodCIDR is the primary alias range the node boots with. It reaches
	// the daemon through the CNI config (pb.IPConfig.InitialPodCidr), not through
	// CCM, which is what makes bootstrap interesting.
	InitialPodCIDR = "10.0.0.0/28"

	// InitialPodCIDRv6 is the default initial IPv6 prefix used for DualStack and IPv6Only modes.
	InitialPodCIDRv6 = "2001:db8:1::/112"

	// InitialUsableIPs: the daemon reserves the first two and the last IP of the first block of a
	// network, so a /28 initial block yields 13 allocatable IPs.
	InitialUsableIPs = 13

	MonitorInterval = 200 * time.Millisecond
	EventuallyTick  = 50 * time.Millisecond
	EventuallyMax   = 15 * time.Second
	// AllocateTimeout must exceed the daemon's scale-up wait (9s) so a blocked
	// allocation surfaces as the daemon's own error rather than a client timeout.
	AllocateTimeout = 20 * time.Second

	// Internal aliases
	nodeName         = NodeName
	zone             = Zone
	project          = Project
	network          = Network
	initialPodCIDR   = InitialPodCIDR
	initialPodCIDRv6 = InitialPodCIDRv6
	initialUsableIPs = InitialUsableIPs
	monitorInterval  = MonitorInterval
	eventuallyTick   = EventuallyTick
	eventuallyMax    = EventuallyMax
	allocateTimeout  = AllocateTimeout
)

var (
	ProviderID  = fmt.Sprintf("gce://%s/%s/%s", Project, Zone, NodeName)
	NetworkURL  = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/default", Project)
	InstanceKey = meta.ZonalKey(NodeName, Zone)

	providerID  = ProviderID
	networkURL  = NetworkURL
	instanceKey = InstanceKey
)

// StackType defines the IP address family configuration of the cluster and node.
type StackType string

const (
	// StackTypeIPv4 allocates and manages only IPv4 addresses.
	StackTypeIPv4 StackType = "IPv4"
	// StackTypeDualStack allocates and manages both IPv4 and IPv6 addresses.
	StackTypeDualStack StackType = "DualStack"
	// StackTypeIPv6Only allocates and manages only IPv6 addresses.
	StackTypeIPv6Only StackType = "IPv6Only"
)

// NodeHandle encapsulates node-specific clients, store, and daemon lifecycle for a node.
type NodeHandle struct {
	name            string
	providerID      string
	instanceKey     *meta.Key
	daemonConfig    daemon.Config
	daemonCtx       context.Context
	daemonCancel    context.CancelFunc
	daemonDone      chan struct{}
	daemonConn      io.Closer
	ipam            pb.AdaptiveIpamClient
	cniPlugin       *cni.Plugin
	cniNetConf      []byte
	initialIPv4CIDR string
	initialIPv6CIDR string
	store           *store.Store
	fixture         *Fixture
}

// Name returns the node's name.
func (n *NodeHandle) Name() string { return n.name }

// ProviderID returns the node's GCE ProviderID.
func (n *NodeHandle) ProviderID() string { return n.providerID }

// Store returns the SQLite store handle for this node.
func (n *NodeHandle) Store() *store.Store { return n.store }

// DaemonConfig returns the daemon's runtime configuration.
func (n *NodeHandle) DaemonConfig() daemon.Config { return n.daemonConfig }

// Fixture returns the parent test fixture.
func (n *NodeHandle) Fixture() *Fixture { return n.fixture }

// nodeHandle is an internal type alias for backward compatibility across fixture files.
type nodeHandle = NodeHandle

// initCNI initializes the node's CNI plugin instance and serializes its standard CNI network config JSON.
func (n *nodeHandle) initCNI() {
	logPath := filepath.Join(filepath.Dir(n.daemonConfig.DBPath), "metis-cni.log")
	n.cniPlugin = cni.NewPlugin(
		cni.WithSocketPath(n.daemonConfig.SocketPath),
		cni.WithDBPath(n.daemonConfig.DBPath),
		cni.WithLogFile(logPath),
	)

	var ranges [][]cni.SubnetRange
	var routes []cni.Route

	if n.fixture.cfg.stackType != StackTypeIPv6Only && n.initialIPv4CIDR != "" {
		_, v4Net, err := net.ParseCIDR(n.initialIPv4CIDR)
		if err != nil {
			n.fixture.t.Fatalf("invalid initial IPv4 CIDR %s on node %s: %v", n.initialIPv4CIDR, n.name, err)
		}
		ranges = append(ranges, []cni.SubnetRange{{Subnet: cnitypes.IPNet(*v4Net)}})
		routes = append(routes, cni.Route{Dst: "0.0.0.0/0"})
	}
	if (n.fixture.cfg.stackType == StackTypeDualStack || n.fixture.cfg.stackType == StackTypeIPv6Only) && n.initialIPv6CIDR != "" {
		_, v6Net, err := net.ParseCIDR(n.initialIPv6CIDR)
		if err != nil {
			n.fixture.t.Fatalf("invalid initial IPv6 CIDR %s on node %s: %v", n.initialIPv6CIDR, n.name, err)
		}
		ranges = append(ranges, []cni.SubnetRange{{Subnet: cnitypes.IPNet(*v6Net)}})
		routes = append(routes, cni.Route{Dst: "::/0"})
	}

	conf := cni.PluginConf{
		PluginConf: cnitypes.PluginConf{
			CNIVersion: "0.3.1",
			Name:       "gke-pod-network",
			Type:       "metis",
		},
		DaemonSocket: n.daemonConfig.SocketPath,
		DBPath:       n.daemonConfig.DBPath,
		LogFile:      logPath,
		IPAM: cni.IPAM{
			Ranges: ranges,
			Routes: routes,
		},
	}

	data, err := json.Marshal(conf)
	if err != nil {
		n.fixture.t.Fatalf("failed to marshal CNI config for node %s: %v", n.name, err)
	}
	n.cniNetConf = data
}

// Fixture orchestrates an integration test environment, coupling simulated K8s
// and GCE backends with live CCM controllers and a running Metis daemon instance.
type Fixture struct {
	t   *testing.T
	ctx context.Context
	cfg FixtureConfig

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

	daemonConfig daemon.Config
	daemonCtx    context.Context
	daemonCancel context.CancelFunc
	daemonDone   chan struct{}
	daemonConn   io.Closer

	nodes map[string]*NodeHandle
}

// fixture is an internal alias for Fixture across fixture files.
type fixture = Fixture

// Ctx returns the test fixture's execution context.
func (f *Fixture) Ctx() context.Context { return f.ctx }

// KubeClient returns the fake Kubernetes clientset.
func (f *Fixture) KubeClient() *k8sfake.Clientset { return f.kubeClient }

// NNCClient returns the fake NodeNetworkConfig clientset.
func (f *Fixture) NNCClient() *nncfake.Clientset { return f.nncClient }

// GCE returns the fake GCE Cloud instance.
func (f *Fixture) GCE() *gce.Cloud { return f.gce }

// FixtureConfig stores configuration options for initializing a test fixture.
type FixtureConfig struct {
	nodeProviderID       string
	backendType          dynamicpodip.BackendType
	stackType            StackType
	initialIPv4CIDR      string
	initialIPv6CIDR      string
	daemonConfigModifier func(*daemon.Config)
	nncReactors          []customReactor
}

// fixtureConfig is an internal alias for FixtureConfig.
type fixtureConfig = FixtureConfig

// FixtureOption configures a FixtureConfig during test fixture initialization.
type FixtureOption func(*FixtureConfig)

// fixtureOption is an internal alias for FixtureOption.
type fixtureOption = FixtureOption

// WithStackType sets the cluster stack configuration (IPv4, DualStack, or IPv6Only).
func WithStackType(st StackType) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.stackType = st
	}
}

// WithInitialAliasIP configures a custom initial IPv4 alias IP range on the primary interface.
func WithInitialAliasIP(cidr string) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.initialIPv4CIDR = cidr
	}
}

// WithInitialPodCIDRv6 configures a custom initial IPv6 CIDR prefix for dual-stack or IPv6-only modes.
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

// WithPrependReactor registers a custom client-go reactor on the fake NNC clientset,
// allowing tests to inject specific API errors (such as 409 Conflict) or intercept actions.
func WithPrependReactor(verb, resource string, reaction clientgotesting.ReactionFunc) FixtureOption {
	return func(cfg *FixtureConfig) {
		cfg.nncReactors = append(cfg.nncReactors, customReactor{
			verb:     verb,
			resource: resource,
			reaction: reaction,
		})
	}
}

var (
	withStackType       = WithStackType
	withInitialAliasIP  = WithInitialAliasIP
	withInitialPodCIDRv6 = WithInitialPodCIDRv6
	withNodeProviderID  = WithNodeProviderID
	withBackend         = WithBackend
	withDaemonConfig    = WithDaemonConfig
	withPrependReactor  = WithPrependReactor
	newFixture          = NewFixture
)

// NewFixture constructs and boots an isolated integration test environment containing
// a simulated K8s cluster with fake core and NNC clientsets, a simulated GCE compute
// environment with mock instance network interfaces, live CCM NodeNetworkConfig spec
// and status controllers, an in-process Metis CNI IPAM plugin instance wired to standard
// CNI network configurations, and a live Metis daemon backed by a real SQLite store and
// gRPC server. All background routines, sockets, and temporary files are automatically
// cleaned up when the test finishes.
func NewFixture(t *testing.T, opts ...FixtureOption) *Fixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := fixtureConfig{
		nodeProviderID:  providerID,
		stackType:       StackTypeIPv4,
		initialIPv4CIDR: initialPodCIDR,
		initialIPv6CIDR: initialPodCIDRv6,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	f := &fixture{
		t:          t,
		ctx:        ctx,
		cfg:        cfg,
		kubeClient: k8sfake.NewSimpleClientset(),
		nncClient:  nncfake.NewSimpleClientset(),
		nodes:      make(map[string]*nodeHandle),
	}

	emulateAPIServerSemantics(f.nncClient)
	for _, r := range cfg.nncReactors {
		f.nncClient.PrependReactor(r.verb, r.resource, r.reaction)
	}

	// --- Node -------------------------------------------------------------------
	if _, err := f.kubeClient.CoreV1().Nodes().Create(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Spec:       corev1.NodeSpec{ProviderID: cfg.nodeProviderID},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create fake Node: %v", err)
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

	var initialAliasRanges []*compute.AliasIpRange
	if cfg.stackType != StackTypeIPv6Only && cfg.initialIPv4CIDR != "" {
		initialAliasRanges = []*compute.AliasIpRange{{IpCidrRange: cfg.initialIPv4CIDR}}
	}

	if err := f.gce.Compute().Instances().Insert(ctx, instanceKey, &compute.Instance{
		Name: nodeName,
		Zone: zone,
		NetworkInterfaces: []*compute.NetworkInterface{{
			Name:          "nic0",
			Network:       networkURL,
			Subnetwork:    "default",
			AliasIpRanges: initialAliasRanges,
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

	daemonCtx, daemonCancel := context.WithCancel(ctx)
	daemonDone := make(chan struct{})
	f.daemonConfig = dConfig
	f.daemonCtx = daemonCtx
	f.daemonCancel = daemonCancel
	f.daemonDone = daemonDone

	d := daemon.NewDaemon(dConfig)
	d.NNCClient = f.nncClient
	d.KubeClient = f.kubeClient
	go func() {
		defer close(daemonDone)
		if err := d.Run(daemonCtx); err != nil && daemonCtx.Err() == nil {
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
	f.daemonConn = conn
	t.Cleanup(func() {
		if f.daemonCancel != nil {
			f.daemonCancel()
		}
		if f.daemonConn != nil {
			_ = f.daemonConn.Close()
		}
	})
	f.ipam = pb.NewAdaptiveIpamClient(conn)

	s, err := store.NewStore(ctx, logr.Discard(), dbPath)
	if err != nil {
		t.Fatalf("failed to open store for assertions: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	f.store = s

	primaryHandle := &nodeHandle{
		name:            nodeName,
		providerID:      providerID,
		instanceKey:     instanceKey,
		daemonConfig:    dConfig,
		daemonCtx:       daemonCtx,
		daemonCancel:    daemonCancel,
		daemonDone:      daemonDone,
		daemonConn:      conn,
		ipam:            f.ipam,
		initialIPv4CIDR: f.cfg.initialIPv4CIDR,
		initialIPv6CIDR: f.cfg.initialIPv6CIDR,
		store:           s,
		fixture:         f,
	}
	primaryHandle.initCNI()
	f.nodes[nodeName] = primaryHandle

	return f
}

// PrimaryNode returns the handle for the cluster's primary node.
func (f *Fixture) PrimaryNode() *NodeHandle {
	return f.nodes[nodeName]
}

func (f *Fixture) primaryNode() *NodeHandle {
	return f.PrimaryNode()
}

// Node returns the handle for a specific node in the cluster by name.
func (f *Fixture) Node(name string) *NodeHandle {
	return f.nodes[name]
}

func (f *Fixture) node(name string) *NodeHandle {
	return f.Node(name)
}

// AddNode spins up an additional simulated Node in the cluster with its own GCE
// instance, its own Metis daemon instance, its own SQLite database, and gRPC client.
func (f *Fixture) AddNode(name string, initialCIDR ...string) *NodeHandle {
	f.t.Helper()
	nodeProviderID := fmt.Sprintf("gce://%s/%s/%s", project, zone, name)
	nodeInstanceKey := meta.ZonalKey(name, zone)

	// Create Node in Kubernetes
	if _, err := f.kubeClient.CoreV1().Nodes().Create(f.ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{ProviderID: nodeProviderID},
	}, metav1.CreateOptions{}); err != nil {
		f.t.Fatalf("failed to create node %s: %v", name, err)
	}

	cidr := fmt.Sprintf("10.0.%d.0/28", len(f.nodes))
	if len(initialCIDR) > 0 && initialCIDR[0] != "" {
		cidr = initialCIDR[0]
	}

	var aliasRanges []*compute.AliasIpRange
	if f.cfg.stackType != StackTypeIPv6Only && cidr != "" {
		aliasRanges = []*compute.AliasIpRange{{IpCidrRange: cidr}}
	}

	if err := f.gce.Compute().Instances().Insert(f.ctx, nodeInstanceKey, &compute.Instance{
		Name: name,
		Zone: zone,
		NetworkInterfaces: []*compute.NetworkInterface{{
			Name:          "nic0",
			Network:       networkURL,
			Subnetwork:    "default",
			AliasIpRanges: aliasRanges,
		}},
	}); err != nil {
		f.t.Fatalf("failed to insert GCE instance for node %s: %v", name, err)
	}

	nodeDir := f.t.TempDir()
	dbPath := filepath.Join(nodeDir, "metis.sqlite")
	socketPath := filepath.Join(nodeDir, "metis.sock")

	dConfig := daemon.Config{
		DBPath:                          dbPath,
		SocketPath:                      socketPath,
		MonitorInterval:                 monitorInterval,
		ReleaseCooldown:                 100 * time.Millisecond,
		DrainingExpiration:              200 * time.Millisecond,
		SustainedLowUtilizationDuration: 100 * time.Millisecond,
		LowUtilizationThreshold:         0.3,
	}

	daemonCtx, daemonCancel := context.WithCancel(f.ctx)
	daemonDone := make(chan struct{})

	// Set NODE_NAME temporarily so the daemon initializes with this node's name
	f.t.Setenv("NODE_NAME", name)

	d := daemon.NewDaemon(dConfig)
	d.NNCClient = f.nncClient
	d.KubeClient = f.kubeClient

	go func() {
		defer close(daemonDone)
		if err := d.Run(daemonCtx); err != nil && daemonCtx.Err() == nil {
			f.t.Errorf("daemon for node %s exited with error: %v", name, err)
		}
	}()

	f.Eventually(fmt.Sprintf("daemon socket for node %s is ready", name), func() (bool, string) {
		_, err := os.Stat(socketPath)
		return err == nil, fmt.Sprintf("stat %s: %v", socketPath, err)
	})

	conn, err := metispkg.NewLocalGrpcConnection(socketPath)
	if err != nil {
		f.t.Fatalf("failed to connect to daemon socket for node %s: %v", name, err)
	}

	s, err := store.NewStore(f.ctx, logr.Discard(), dbPath)
	if err != nil {
		f.t.Fatalf("failed to open store handle for node %s: %v", name, err)
	}

	nh := &nodeHandle{
		name:            name,
		providerID:      nodeProviderID,
		instanceKey:     nodeInstanceKey,
		daemonConfig:    dConfig,
		daemonCtx:       daemonCtx,
		daemonCancel:    daemonCancel,
		daemonDone:      daemonDone,
		daemonConn:      conn,
		ipam:            pb.NewAdaptiveIpamClient(conn),
		initialIPv4CIDR: cidr,
		initialIPv6CIDR: f.cfg.initialIPv6CIDR,
		store:           s,
		fixture:         f,
	}
	nh.initCNI()

	f.t.Cleanup(func() {
		if nh.daemonCancel != nil {
			nh.daemonCancel()
		}
		if nh.daemonConn != nil {
			_ = nh.daemonConn.Close()
		}
		if nh.store != nil {
			_ = nh.store.Close()
		}
	})

	f.nodes[name] = nh
	return nh
}

func (f *Fixture) addNode(name string, initialCIDR ...string) *NodeHandle {
	return f.AddNode(name, initialCIDR...)
}

// RestartDaemon stops the running daemon process, closes its gRPC connection,
// starts a fresh daemon against the existing SQLite DB and NNC, and reconnects.
func (f *Fixture) RestartDaemon() {
	f.t.Helper()
	if f.daemonCancel != nil {
		f.daemonCancel()
	}
	if f.daemonDone != nil {
		<-f.daemonDone
	}
	if f.daemonConn != nil {
		_ = f.daemonConn.Close()
	}

	daemonCtx, daemonCancel := context.WithCancel(f.ctx)
	daemonDone := make(chan struct{})
	f.daemonCtx = daemonCtx
	f.daemonCancel = daemonCancel
	f.daemonDone = daemonDone

	d := daemon.NewDaemon(f.daemonConfig)
	d.NNCClient = f.nncClient
	d.KubeClient = f.kubeClient
	go func() {
		defer close(daemonDone)
		if err := d.Run(daemonCtx); err != nil && daemonCtx.Err() == nil {
			f.t.Errorf("restarted daemon exited with error: %v", err)
		}
	}()

	f.Eventually("restarted daemon socket is ready", func() (bool, string) {
		_, err := os.Stat(f.daemonConfig.SocketPath)
		return err == nil, fmt.Sprintf("stat %s: %v", f.daemonConfig.SocketPath, err)
	})

	conn, err := metispkg.NewLocalGrpcConnection(f.daemonConfig.SocketPath)
	if err != nil {
		f.t.Fatalf("failed to connect to restarted daemon socket: %v", err)
	}
	f.daemonConn = conn
	f.ipam = pb.NewAdaptiveIpamClient(conn)
	if pNode, ok := f.nodes[nodeName]; ok {
		pNode.daemonCtx = daemonCtx
		pNode.daemonCancel = daemonCancel
		pNode.daemonDone = daemonDone
		pNode.daemonConn = conn
		pNode.ipam = f.ipam
	}
}

func (f *Fixture) restartDaemon() {
	f.RestartDaemon()
}

// Eventually polls cond until it returns true or eventuallyMax elapses.
// If the deadline expires, the test fails with the last non-empty detail.
func (f *Fixture) Eventually(desc string, cond func() (bool, string)) {
	f.t.Helper()
	var lastDetail string
	err := wait.PollUntilContextTimeout(f.ctx, eventuallyTick, eventuallyMax, true, func(ctx context.Context) (bool, error) {
		ok, detail := cond()
		if !ok {
			lastDetail = detail
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		f.t.Fatalf("timed out waiting for %s; last observation: %s", desc, lastDetail)
	}
}

func (f *Fixture) eventually(desc string, cond func() (bool, string)) {
	f.Eventually(desc, cond)
}

// Consistently asserts that cond remains true for the full duration d, sampled
// every eventuallyTick. Useful for verifying that a quiescent state does not leak.
func (f *Fixture) Consistently(desc string, d time.Duration, cond func() (bool, string)) {
	f.t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		ok, detail := cond()
		if !ok {
			f.t.Fatalf("consistently failed for %s: %s", desc, detail)
		}
		select {
		case <-f.ctx.Done():
			f.t.Fatalf("context cancelled during consistently(%s)", desc)
		case <-time.After(eventuallyTick):
		}
	}
}

func (f *Fixture) consistently(desc string, d time.Duration, cond func() (bool, string)) {
	f.Consistently(desc, d, cond)
}
