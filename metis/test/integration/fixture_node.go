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

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/go-logr/logr"
	compute "google.golang.org/api/compute/v1"
	computepreview "google.golang.org/api/compute/v1.preview"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pb "k8s.io/metis/api/adaptiveipam/v1"
	metispkg "k8s.io/metis/pkg"
	"k8s.io/metis/pkg/cni"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/pkg/store"
)

// Node defines operations available on any simulated cluster node.
type Node interface {
	// Identity and metadata (fixture_node.go)
	Name() string
	ProviderID() string
	InstanceKey() *meta.Key
	Fixture() *Fixture

	// Node lifecycle and runtime handles (fixture_node.go)
	Store() *store.Store
	IPAMClient() pb.AdaptiveIpamClient
	CNIPlugin() *cni.Plugin
	CNINetConf() []byte
	DaemonConfig() daemon.Config
	RestartDaemon()
	Close()

	// Pod and IPAM operations (fixture_ipam.go)
	AllocatePod(id string) (ip, cidr string)
	AllocateDualStackPod(id string) (ipv4, cidrv4, ipv6, cidrv6 string)
	TryAllocatePod(id string) (cidr string, err error)
	DeletePod(id string)
	Usage() store.NetworkIPUsage
	IPv6Usage() store.NetworkIPUsage
	BlockState(cidr string) store.CidrBlockState
	IPv6BlockState(cidr string) store.CidrBlockState
	BlockExists(cidr string) bool
	BlockAllocatedIPs(cidr string) int

	// Kubernetes NodeNetworkConfig operations (fixture_nnc.go)
	NNC() *nncv1.NodeNetworkConfig
	SettledSpecPods() int32
	AddReleasableCIDR(cidr string)
	SetAllocationsPods(pods int32)
	NodeEvents(reason string) []string

	// Cloud provider operations (fixture_gce.go)
	GCEAliasRanges() []string
	GCEANEs() []*computepreview.AliasNetworkEndpoint
	GCECIDRs() []string
}

// nodeHandle implements Node for a simulated node in the integration cluster.
type nodeHandle struct {
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

// Compile-time interface check:
// Because nodeHandle methods are implemented across domain-specific files
// (fixture_node.go, fixture_ipam.go, fixture_nnc.go, fixture_gce.go), this
// compile-time assertion ensures that any additions or changes to the Node
// interface remain fully implemented across all files.
var _ Node = (*nodeHandle)(nil)

// Name returns the node's name.
func (n *nodeHandle) Name() string { return n.name }

// ProviderID returns the node's GCE ProviderID.
func (n *nodeHandle) ProviderID() string { return n.providerID }

// InstanceKey returns the zonal compute metadata key for this node.
func (n *nodeHandle) InstanceKey() *meta.Key { return n.instanceKey }

// Store returns the SQLite store handle for this node.
func (n *nodeHandle) Store() *store.Store { return n.store }

// IPAMClient returns the gRPC client connection to this node's Metis daemon.
func (n *nodeHandle) IPAMClient() pb.AdaptiveIpamClient { return n.ipam }

// CNIPlugin returns the CNI plugin instance wired for this node.
func (n *nodeHandle) CNIPlugin() *cni.Plugin { return n.cniPlugin }

// CNINetConf returns the serialized CNI network configuration JSON.
func (n *nodeHandle) CNINetConf() []byte { return n.cniNetConf }

// DaemonConfig returns the daemon's runtime configuration.
func (n *nodeHandle) DaemonConfig() daemon.Config { return n.daemonConfig }

// Fixture returns the parent test fixture.
func (n *nodeHandle) Fixture() *Fixture { return n.fixture }

// initCNI initializes the node's CNI plugin instance and serializes its
// standard CNI network config JSON.
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

// Close terminates the node's daemon goroutine, closes its gRPC connection,
// and releases its SQLite store handle.
func (n *nodeHandle) Close() {
	if n.daemonCancel != nil {
		n.daemonCancel()
		n.daemonCancel = nil
	}
	if n.daemonDone != nil {
		<-n.daemonDone
		n.daemonDone = nil
	}
	if n.daemonConn != nil {
		_ = n.daemonConn.Close()
		n.daemonConn = nil
	}
	if n.store != nil {
		_ = n.store.Close()
		n.store = nil
	}
}

// RestartDaemon stops the node's running daemon, closes its gRPC connection,
// starts a fresh daemon process against the existing SQLite DB and NNC, and
// reconnects.
func (n *nodeHandle) RestartDaemon() {
	n.fixture.t.Helper()
	if n.daemonCancel != nil {
		n.daemonCancel()
	}
	if n.daemonDone != nil {
		<-n.daemonDone
	}
	if n.daemonConn != nil {
		_ = n.daemonConn.Close()
	}

	daemonCtx, daemonCancel := context.WithCancel(n.fixture.ctx)
	daemonDone := make(chan struct{})
	n.daemonCtx = daemonCtx
	n.daemonCancel = daemonCancel
	n.daemonDone = daemonDone

	d := daemon.NewDaemon(n.daemonConfig)
	d.NNCClient = n.fixture.nncClient
	d.KubeClient = n.fixture.kubeClient
	go func() {
		defer close(daemonDone)
		if err := d.Run(daemonCtx); err != nil && daemonCtx.Err() == nil {
			n.fixture.t.Errorf("restarted daemon for node %s exited with error: %v", n.name, err)
		}
	}()

	n.fixture.Eventually(fmt.Sprintf("restarted daemon socket for node %s is ready", n.name), func() (bool, string) {
		_, err := os.Stat(n.daemonConfig.SocketPath)
		return err == nil, fmt.Sprintf("stat %s: %v", n.daemonConfig.SocketPath, err)
	})

	conn, err := metispkg.NewLocalGrpcConnection(n.daemonConfig.SocketPath)
	if err != nil {
		n.fixture.t.Fatalf("failed to connect to restarted daemon socket for node %s: %v", n.name, err)
	}
	n.daemonConn = conn
	n.ipam = pb.NewAdaptiveIpamClient(conn)
}

// FirstNode returns the handle for the cluster's first (initial) node.
func (f *Fixture) FirstNode() Node {
	return f.Node
}

// NodeNamed returns the handle for a specific node in the cluster by name.
func (f *Fixture) NodeNamed(name string) Node {
	return f.nodes[name]
}

// AddNode spins up an additional simulated Node in the cluster with its own
// GCE instance, its own Metis daemon instance, its own SQLite database, and
// gRPC client.
func (f *Fixture) AddNode(name string, initialCIDR ...string) Node {
	f.t.Helper()
	cidr := fmt.Sprintf("10.0.%d.0/28", len(f.nodes))
	if len(initialCIDR) > 0 && initialCIDR[0] != "" {
		cidr = initialCIDR[0]
	}

	return f.bootNode(name, cidr, f.cfg.initialIPv6CIDR, nil)
}

// bootNode initializes and boots a node in the test environment: creates the
// K8s Node and GCE Instance resources if needed, launches its Metis daemon
// with dedicated storage and socket, connects via gRPC, initializes CNI
// config, and registers cleanup hooks.
//
// Note on future multi-networking extension:
// The integration harness currently provisions a single primary network
// interface (nic0 attached to GCENetworkURL("default")). When end-to-end
// multi-networking test scenarios are introduced, this harness can be
// extended with a NetworkInterfaceConfig descriptor on FixtureConfig and
// Node, allowing nodes to bind multiple network interfaces (nic0,
// nic1, etc.) across distinct VPC networks, and parameterizing
// GCEAliasRanges(iface...) and Usage(network...).
func (f *Fixture) bootNode(
	name string,
	initialIPv4 string,
	initialIPv6 string,
	dConfigModifier func(*daemon.Config),
) *nodeHandle {
	f.t.Helper()

	instanceKey := NodeInstanceKey(name)

	// Ensure fake Node exists in Kubernetes.
	node, err := f.kubeClient.CoreV1().Nodes().Get(f.ctx, name, metav1.GetOptions{})
	if err != nil {
		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.NodeSpec{ProviderID: NodeProviderID(name)},
		}
		if _, err := f.kubeClient.CoreV1().Nodes().Create(f.ctx, node, metav1.CreateOptions{}); err != nil {
			f.t.Fatalf("failed to create fake Node %s: %v", name, err)
		}
	}
	providerID := node.Spec.ProviderID

	// Ensure fake GCE Instance exists.
	var aliasRanges []*compute.AliasIpRange
	if f.cfg.stackType != StackTypeIPv6Only && initialIPv4 != "" {
		aliasRanges = []*compute.AliasIpRange{{IpCidrRange: initialIPv4}}
	}
	if _, err := f.gce.Compute().Instances().Get(f.ctx, instanceKey); err != nil {
		if err := f.gce.Compute().Instances().Insert(f.ctx, instanceKey, &compute.Instance{
			Name: name,
			Zone: defaultZone,
			NetworkInterfaces: []*compute.NetworkInterface{{
				Name:          "nic0",
				Network:       GCENetworkURL("default"),
				Subnetwork:    "default",
				AliasIpRanges: aliasRanges,
			}},
		}); err != nil {
			f.t.Fatalf("failed to insert fake GCE instance for %s: %v", name, err)
		}
	}

	// Provision isolated node storage and socket.
	dir := f.t.TempDir()
	dbPath := filepath.Join(dir, "metis.sqlite")
	socketPath := filepath.Join(dir, "metis.sock")
	f.t.Setenv("NODE_NAME", name)

	dConfig := daemon.Config{
		DBPath:                          dbPath,
		SocketPath:                      socketPath,
		MonitorInterval:                 MonitorInterval,
		ReleaseCooldown:                 100 * time.Millisecond,
		DrainingExpiration:              200 * time.Millisecond,
		SustainedLowUtilizationDuration: 100 * time.Millisecond,
		LowUtilizationThreshold:         0.3,
	}
	if dConfigModifier != nil {
		dConfigModifier(&dConfig)
	}

	daemonCtx, daemonCancel := context.WithCancel(f.ctx)
	daemonDone := make(chan struct{})

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
		providerID:      providerID,
		instanceKey:     instanceKey,
		daemonConfig:    dConfig,
		daemonCtx:       daemonCtx,
		daemonCancel:    daemonCancel,
		daemonDone:      daemonDone,
		daemonConn:      conn,
		ipam:            pb.NewAdaptiveIpamClient(conn),
		initialIPv4CIDR: initialIPv4,
		initialIPv6CIDR: initialIPv6,
		store:           s,
		fixture:         f,
	}
	nh.initCNI()

	f.t.Cleanup(nh.Close)
	f.nodes[name] = nh
	return nh
}
