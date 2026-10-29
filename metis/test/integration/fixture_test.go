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

// Package integration tests the metis daemon and the CCM dynamic pod IP
// controllers end-to-end against fake GCE and fake Kubernetes apiservers.
//
// The harness exercises the full loop:
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
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
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

	daemonConfig daemon.Config
	daemonCtx    context.Context
	daemonCancel context.CancelFunc
	daemonDone   chan struct{}
	daemonConn   io.Closer
}

type fixtureConfig struct {
	nodeProviderID       string
	backendType          dynamicpodip.BackendType
	daemonConfigModifier func(*daemon.Config)
	nncReactors          []customReactor
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

func newFixture(t *testing.T, opts ...fixtureOption) *fixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := fixtureConfig{
		nodeProviderID: providerID,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	f := &fixture{
		t:          t,
		ctx:        ctx,
		kubeClient: k8sfake.NewSimpleClientset(),
		nncClient:  nncfake.NewSimpleClientset(),
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

	return f
}

// restartDaemon stops the running daemon process, closes its gRPC connection,
// starts a fresh daemon against the existing SQLite DB and NNC, and reconnects.
func (f *fixture) restartDaemon() {
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

	f.eventually("restarted daemon socket is ready", func() (bool, string) {
		_, err := os.Stat(f.daemonConfig.SocketPath)
		return err == nil, fmt.Sprintf("stat %s: %v", f.daemonConfig.SocketPath, err)
	})

	conn, err := metispkg.NewLocalGrpcConnection(f.daemonConfig.SocketPath)
	if err != nil {
		f.t.Fatalf("failed to connect to restarted daemon socket: %v", err)
	}
	f.daemonConn = conn
	f.ipam = pb.NewAdaptiveIpamClient(conn)
}

// eventually polls cond until it returns true or eventuallyMax elapses.
// If the deadline expires, the test fails with the last non-empty detail.
func (f *fixture) eventually(desc string, cond func() (bool, string)) {
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

// consistently asserts that cond remains true for the full duration d, sampled
// every eventuallyTick. Useful for verifying that a quiescent state does not leak.
func (f *fixture) consistently(desc string, d time.Duration, cond func() (bool, string)) {
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
