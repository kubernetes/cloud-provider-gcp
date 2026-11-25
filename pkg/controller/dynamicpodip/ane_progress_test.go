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

package dynamicpodip

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	computepreview "google.golang.org/api/compute/v1.preview"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/utils/clock"
)

// receive returns the next value sent on ch, failing the test if nothing
// arrives in time.
func receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		var zero T
		return zero
	}
}

// expectEmpty fails the test if a value is already waiting on ch.
func expectEmpty[T any](t *testing.T, ch <-chan T, problem string) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("%s (got %v)", problem, v)
	default:
	}
}

func TestProgressDebouncer(t *testing.T) {
	const interval = time.Second
	start := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

	calls := 0
	d := progressDebouncer{notify: func() { calls++ }, interval: interval}

	// flushAt flushes at the given offset from start, and checks how many
	// completions that reported and how many calls there have been in all.
	flushAt := func(step string, offset time.Duration, wantCovered, wantCalls int) {
		t.Helper()
		if got := d.flush(start.Add(offset)); got != wantCovered {
			t.Fatalf("%s: flush reported %d completion(s), want %d", step, got, wantCovered)
		}
		if calls != wantCalls {
			t.Fatalf("%s: %d call(s) so far, want %d", step, calls, wantCalls)
		}
	}

	flushAt("nothing has completed", 0, 0, 0)

	d.completed()
	flushAt("the first completion is reported at once", 0, 1, 1)
	flushAt("there is nothing new to report", 100*time.Millisecond, 0, 1)

	d.completed()
	d.completed()
	flushAt("completions inside the interval are held back", 400*time.Millisecond, 0, 1)
	flushAt("up to the very end of it", interval-time.Nanosecond, 0, 1)
	flushAt("and are then reported together", interval, 2, 2)

	d.completed()
	flushAt("the interval is measured from the last report", interval+500*time.Millisecond, 0, 2)
	flushAt("so the next report waits a full interval again", 2*interval, 1, 3)

	d.completed()
	flushAt("a completion after a quiet period is reported at once", 10*interval, 1, 4)
}

func TestProgressDebouncer_NilCallback(t *testing.T) {
	d := progressDebouncer{interval: time.Second}
	d.completed()
	if got := d.flush(time.Now()); got != 0 {
		t.Fatalf("flush with no callback reported %d completion(s), want 0", got)
	}
}

// stagedANECloud wraps the fake cloud so that a test decides when each
// endpoint creation completes. Until it does, the endpoint is listed as
// CREATING, which is how GCE lists an insert whose operation is still
// running.
type stagedANECloud struct {
	*gce.Cloud

	// started receives an endpoint's name once it is listed as CREATING and
	// its creation is waiting to be released.
	started chan string
	// release lets one waiting creation complete for each value sent.
	release chan struct{}
	// stop is closed when the test ends, so that a creation still waiting
	// after a failed assertion does not outlive it.
	stop chan struct{}
}

func newStagedANECloud(t *testing.T, fakeGCE *gce.Cloud) *stagedANECloud {
	c := &stagedANECloud{
		Cloud:   fakeGCE,
		started: make(chan string, 16),
		release: make(chan struct{}),
		stop:    make(chan struct{}),
	}
	t.Cleanup(func() { close(c.stop) })
	return c
}

func (c *stagedANECloud) CreateAliasNetworkEndpoint(ctx context.Context, providerID string, endpoint *computepreview.AliasNetworkEndpoint) error {
	creating := *endpoint
	creating.Status = &computepreview.AliasNetworkEndpointStatus{State: gce.ANEStateCreating}
	if err := c.Cloud.CreateAliasNetworkEndpoint(ctx, providerID, &creating); err != nil {
		return err
	}
	c.started <- endpoint.Name

	select {
	case <-c.release:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stop:
		return errors.New("the test ended while this creation was still in flight")
	}

	// The fake cannot move an endpoint from CREATING to ACTIVE, so replace
	// it. Nothing reads the instance during the swap: the tests only do so
	// once the dispatcher has reacted to this call returning.
	if err := c.Cloud.DeleteAliasNetworkEndpoint(ctx, providerID, endpoint.Name); err != nil {
		return err
	}
	return c.Cloud.CreateAliasNetworkEndpoint(ctx, providerID, endpoint)
}

func TestANEBackendMutateWithTopUp_ReportsProgressWhileOthersAreInFlight(t *testing.T) {
	cloud := newStagedANECloud(t, newFakeGCEWithDefaultInstance(t))
	backend := &aneBackend{gceCloud: cloud}

	reported := make(chan struct{}, 16)
	done := make(chan error, 1)
	go func() {
		done <- backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32", "/32"}, nil, nil, nil,
			func() { reported <- struct{}{} })
	}()

	receive(t, cloud.started, "the first creation to start")
	receive(t, cloud.started, "the second creation to start")
	expectEmpty(t, reported, "progress was reported before any endpoint had been created")

	// One endpoint is created while the other is still in flight. That is
	// the case the caller has no other way of finding out about.
	cloud.release <- struct{}{}
	receive(t, reported, "progress to be reported with one creation still in flight")

	// The other one leaves nothing in flight. The caller learns that from
	// the call returning, so it is not reported as progress.
	cloud.release <- struct{}{}
	if err := receive(t, done, "the mutation to return"); err != nil {
		t.Fatalf("MutateWithTopUp: %v", err)
	}
	expectEmpty(t, reported, "progress was reported for the creation that completed the batch")
	if got := countANEs(t, backend); got != 2 {
		t.Fatalf("expected 2 ANEs, got %d", got)
	}
}

func TestANEBackendMutateWithTopUp_DoesNotReportASingleCreation(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)

	// The callback runs on the goroutine that drives the mutation, which
	// here is this one, so a plain counter is enough.
	calls := 0
	err := backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32"}, nil, nil, nil, func() { calls++ })
	if err != nil {
		t.Fatalf("MutateWithTopUp: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no progress report when nothing else was in flight, got %d", calls)
	}
}

func TestANEBackendMutateWithTopUp_ReportsWhatCompletedBeforeATopUp(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)

	// The single initial endpoint completes with nothing else in flight, so
	// it is not reported then. The top-up that follows puts operations back
	// in flight, and with that the first endpoint is one worth reporting.
	more, _ := scriptedDemand(2)
	calls := 0
	err := backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32"}, nil, nil, more, func() { calls++ })
	if err != nil {
		t.Fatalf("MutateWithTopUp: %v", err)
	}
	if got := countANEs(t, backend); got != 3 {
		t.Fatalf("expected 1 initial + 2 topped-up = 3 ANEs, got %d", got)
	}
	if calls == 0 {
		t.Fatalf("expected the endpoint created before the top-up to be reported once the top-up was in flight")
	}
}

// triggerRecorder is a StatusTrigger that records requests instead of acting
// on them, so that a test runs the status sync itself, at a point it chooses.
type triggerRecorder chan string

func (r triggerRecorder) EnqueueNode(nodeName string) { r <- nodeName }

// TestSpecControllerPublishesANEsWhileBatchIsInFlight runs both controllers
// against a batch of two endpoints that are created one after the other, and
// checks that the first is published while the second is still CREATING.
func TestSpecControllerPublishesANEsWhileBatchIsInFlight(t *testing.T) {
	ctx := context.Background()

	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
	nodeInformer := informers.NewSharedInformerFactory(kubeClient, 0).Core().V1().Nodes()
	nncInformer := nncinformers.NewSharedInformerFactory(nncClient, 0).Networking().V1().NodeNetworkConfigs()

	cloud := newStagedANECloud(t, newFakeGCEWithDefaultInstance(t))
	backend := &aneBackend{gceCloud: cloud}
	gceCache := NewGCECache(func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		return backend.GetNetworkInterfaces(ctx, providerID)
	}, 10*time.Second, clock.RealClock{})

	statusCtrl := NewStatusController(kubeClient, nncClient, nncInformer.Lister(), nodeInformer, cloud.Cloud, gceCache, clock.RealClock{})
	triggered := make(triggerRecorder, 16)
	specCtrl := NewSpecController(kubeClient, nncClient, nncInformer, nodeInformer, cloud.Cloud, gceCache, triggered, nil, backend)

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}, Spec: corev1.NodeSpec{ProviderID: testProviderID}}
	if _, err := kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Node: %v", err)
	}
	if err := nodeInformer.Informer().GetStore().Add(node); err != nil {
		t.Fatalf("Add Node to cache: %v", err)
	}
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName, Generation: 1},
		Spec:       nncv1.NodeNetworkConfigSpec{Allocations: []nncv1.Allocation{{Network: "default", Pods: 2}}},
	}
	if _, err := nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create NNC: %v", err)
	}
	if err := nncInformer.Informer().GetStore().Add(nnc); err != nil {
		t.Fatalf("Add NNC to cache: %v", err)
	}

	getNNC := func() *nncv1.NodeNetworkConfig {
		t.Helper()
		live, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Get NNC: %v", err)
		}
		return live
	}
	// The informers are not running, so stand in for them: the status
	// controller starts from the object as the API server has it, as it
	// would shortly after the spec controller's write.
	syncStatus := func() error {
		t.Helper()
		if err := nncInformer.Informer().GetStore().Update(getNNC()); err != nil {
			t.Fatalf("Update NNC in cache: %v", err)
		}
		return statusCtrl.syncNode(testNodeName)
	}
	readyCondition := func(live *nncv1.NodeNetworkConfig) metav1.Condition {
		t.Helper()
		cond := getCondition(live.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
		if cond == nil {
			t.Fatalf("NNC has no Ready condition: %+v", live.Status.Conditions)
		}
		return *cond
	}

	specDone := make(chan error, 1)
	go func() { specDone <- specCtrl.syncNode(testNodeName) }()

	receive(t, cloud.started, "the first creation to start")
	receive(t, cloud.started, "the second creation to start")
	expectEmpty(t, triggered, "a status sync was requested before any endpoint had been created")

	// The first endpoint becomes ACTIVE while the second is still CREATING.
	cloud.release <- struct{}{}
	receive(t, triggered, "a status sync to be requested while the batch is in flight")

	err := syncStatus()
	if _, ok := err.(*transientANEError); !ok {
		t.Fatalf("expected the status sync to ask to be requeued for the endpoint still being created, got %T: %v", err, err)
	}
	midBatch := getNNC()
	if len(midBatch.Status.PodCIDRs) != 1 {
		t.Fatalf("expected the one ACTIVE endpoint to be published while the other is being created, got %d: %v", len(midBatch.Status.PodCIDRs), midBatch.Status.PodCIDRs)
	}
	first := midBatch.Status.PodCIDRs[0]
	if first.Condition == nil || first.Condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected the published PodCIDR %q to be Ready, so that the node daemon uses it: %v", first.CIDR, first.Condition)
	}
	if ready := readyCondition(midBatch); ready.Status != metav1.ConditionFalse || ready.Reason != nncReasonUpdating {
		t.Fatalf("expected the node to stay Ready=False/%s while the batch is in flight, got %s/%s", nncReasonUpdating, ready.Status, ready.Reason)
	}

	// The second endpoint completes, which ends the batch.
	cloud.release <- struct{}{}
	if err := receive(t, specDone, "the spec sync to return"); err != nil {
		t.Fatalf("SpecCtrl.syncNode: %v", err)
	}
	receive(t, triggered, "a status sync to be requested at the end of the batch")
	expectEmpty(t, triggered, "more status syncs were requested than one for progress and one for the end of the batch")

	if err := syncStatus(); err != nil {
		t.Fatalf("StatusCtrl.syncNode after the batch: %v", err)
	}
	settled := getNNC()
	if len(settled.Status.PodCIDRs) != 2 {
		t.Fatalf("expected both endpoints to be published after the batch, got %d: %v", len(settled.Status.PodCIDRs), settled.Status.PodCIDRs)
	}
	if ready := readyCondition(settled); ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected the node to be Ready once nothing is in flight, got %s/%s", ready.Status, ready.Reason)
	}
	for _, pc := range settled.Status.PodCIDRs {
		if pc.CIDR == first.CIDR && !pc.Condition.LastTransitionTime.Equal(&first.Condition.LastTransitionTime) {
			t.Errorf("PodCIDR %q was published mid-batch at %v but is restamped %v after it", pc.CIDR, first.Condition.LastTransitionTime, pc.Condition.LastTransitionTime)
		}
	}
}

// aneStatusHarness runs a status controller against the ANEs of the fake
// cloud, with the test standing in for both GCE and the spec controller.
type aneStatusHarness struct {
	t          *testing.T
	fakeGCE    *gce.Cloud
	nncClient  *nncfake.Clientset
	nncStore   cache.Store
	statusCtrl *NodeNetworkConfigStatusController
}

func newANEStatusHarness(t *testing.T) *aneStatusHarness {
	t.Helper()

	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
	nodeInformer := informers.NewSharedInformerFactory(kubeClient, 0).Core().V1().Nodes()
	nncInformer := nncinformers.NewSharedInformerFactory(nncClient, 0).Networking().V1().NodeNetworkConfigs()

	fakeGCE := newFakeGCEWithDefaultInstance(t)
	backend := NewANEBackend(fakeGCE)
	gceCache := NewGCECache(func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		return backend.GetNetworkInterfaces(ctx, providerID)
	}, 10*time.Second, clock.RealClock{})

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}, Spec: corev1.NodeSpec{ProviderID: testProviderID}}
	if err := nodeInformer.Informer().GetStore().Add(node); err != nil {
		t.Fatalf("Add Node to cache: %v", err)
	}
	nnc := &nncv1.NodeNetworkConfig{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
	if _, err := nncClient.NetworkingV1().NodeNetworkConfigs().Create(context.Background(), nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create NNC: %v", err)
	}

	return &aneStatusHarness{
		t:          t,
		fakeGCE:    fakeGCE,
		nncClient:  nncClient,
		nncStore:   nncInformer.Informer().GetStore(),
		statusCtrl: NewStatusController(kubeClient, nncClient, nncInformer.Lister(), nodeInformer, fakeGCE, gceCache, clock.RealClock{}),
	}
}

// setANE puts an endpoint with the given address on the instance in the
// given state, replacing any endpoint of the same name.
func (h *aneStatusHarness) setANE(name, ip, state string) {
	h.t.Helper()
	h.removeANE(name)
	err := h.fakeGCE.CreateAliasNetworkEndpoint(context.Background(), testProviderID, &computepreview.AliasNetworkEndpoint{
		Name:       name,
		Subnetwork: h.fakeGCE.SubnetworkURL(),
		Status:     &computepreview.AliasNetworkEndpointStatus{State: state},
		Aliases: map[string]computepreview.AliasNetworkEndpointAlias{
			DefaultANEAliasNameIPv4: {EffectiveIpAddress: ip},
		},
	})
	if err != nil {
		h.t.Fatalf("Create ANE %q (%s): %v", name, state, err)
	}
}

func (h *aneStatusHarness) removeANE(name string) {
	h.t.Helper()
	if err := h.fakeGCE.DeleteAliasNetworkEndpoint(context.Background(), testProviderID, name); err != nil {
		h.t.Fatalf("Delete ANE %q: %v", name, err)
	}
}

func (h *aneStatusHarness) get() *nncv1.NodeNetworkConfig {
	h.t.Helper()
	live, err := h.nncClient.NetworkingV1().NodeNetworkConfigs().Get(context.Background(), testNodeName, metav1.GetOptions{})
	if err != nil {
		h.t.Fatalf("Get NNC: %v", err)
	}
	return live
}

func (h *aneStatusHarness) updateStatus(mutate func(*nncv1.NodeNetworkConfig)) {
	h.t.Helper()
	live := h.get()
	mutate(live)
	if _, err := h.nncClient.NetworkingV1().NodeNetworkConfigs().UpdateStatus(context.Background(), live, metav1.UpdateOptions{}); err != nil {
		h.t.Fatalf("UpdateStatus: %v", err)
	}
}

// setReady overwrites the Ready condition, as the spec controller does
// before it mutates GCE and when a mutation has failed.
func (h *aneStatusHarness) setReady(status metav1.ConditionStatus, reason, message string) {
	h.t.Helper()
	h.updateStatus(func(live *nncv1.NodeNetworkConfig) {
		setNNCCondition(live, string(nncv1.NodeNetworkConfigConditionReady), status, reason, message)
	})
}

// sync runs one status sync. The informers are not running, so it first
// stands in for them: the controller starts from the object as the API
// server has it.
func (h *aneStatusHarness) sync() error {
	h.t.Helper()
	if err := h.nncStore.Update(h.get()); err != nil {
		h.t.Fatalf("Update NNC in cache: %v", err)
	}
	return h.statusCtrl.syncNode(testNodeName)
}

// syncSettled runs a status sync that is expected to find nothing in flight.
func (h *aneStatusHarness) syncSettled(step string) {
	h.t.Helper()
	if err := h.sync(); err != nil {
		h.t.Fatalf("%s: StatusCtrl.syncNode: %v", step, err)
	}
}

// syncInFlight runs a status sync that is expected to ask to be requeued
// because ANE operations are in flight.
func (h *aneStatusHarness) syncInFlight(step string) {
	h.t.Helper()
	err := h.sync()
	if _, ok := err.(*transientANEError); !ok {
		h.t.Fatalf("%s: expected *transientANEError so that the node is requeued, got %T: %v", step, err, err)
	}
}

// statusWrites counts the status updates made so far, by anyone.
func (h *aneStatusHarness) statusWrites() int {
	n := 0
	for _, action := range h.nncClient.Actions() {
		if action.GetVerb() == "update" && action.GetSubresource() == "status" {
			n++
		}
	}
	return n
}

// expectStatus checks the published ranges and the Ready condition.
func (h *aneStatusHarness) expectStatus(step string, wantCIDRs []string, wantReady metav1.ConditionStatus, wantReason string) {
	h.t.Helper()
	live := h.get()

	var got []string
	for _, pc := range live.Status.PodCIDRs {
		got = append(got, pc.CIDR)
		if pc.Condition == nil || pc.Condition.Status != metav1.ConditionTrue {
			h.t.Errorf("%s: PodCIDR %q is published without a Ready=True condition: %v", step, pc.CIDR, pc.Condition)
		}
	}
	sort.Strings(got)
	want := append([]string(nil), wantCIDRs...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		h.t.Fatalf("%s: published PodCIDRs are %v, want %v", step, got, want)
	}

	ready := getCondition(live.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
	if ready == nil || ready.Status != wantReady || ready.Reason != wantReason {
		h.t.Fatalf("%s: Ready condition is %+v, want %s/%s", step, ready, wantReady, wantReason)
	}
}

const (
	testANEA = "10.128.0.11"
	testANEB = "10.128.0.12"
	testANEC = "10.128.0.13"
)

func TestStatusController_PublishesActiveANEsWhileOthersAreInFlight(t *testing.T) {
	h := newANEStatusHarness(t)
	readyReason := string(nncv1.NodeNetworkConfigReadyReason)

	h.setANE("ane-a", testANEA, gce.ANEStateActive)
	h.syncSettled("one endpoint, nothing in flight")
	h.expectStatus("one endpoint, nothing in flight", []string{testANEA + "/32"}, metav1.ConditionTrue, readyReason)

	// The spec controller starts a batch of two: it marks the node as
	// Updating, and one of the two endpoints is created before the other.
	const updatingMessage = "Updating GCE VM IP alias ranges"
	h.setReady(metav1.ConditionFalse, nncReasonUpdating, updatingMessage)
	h.setANE("ane-b", testANEB, gce.ANEStateActive)
	h.setANE("ane-c", testANEC, gce.ANEStateCreating)

	h.syncInFlight("one of two new endpoints is ACTIVE")
	h.expectStatus("one of two new endpoints is ACTIVE", []string{testANEA + "/32", testANEB + "/32"}, metav1.ConditionFalse, nncReasonUpdating)
	if ready := getCondition(h.get().Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady)); ready.Message != updatingMessage {
		t.Errorf("expected the spec controller's condition to be left as it was, but its message is now %q", ready.Message)
	}

	// Asked again before anything has changed, there is nothing to write.
	writes := h.statusWrites()
	h.syncInFlight("nothing new is ACTIVE")
	if got := h.statusWrites(); got != writes {
		t.Errorf("expected no status write when nothing new is ACTIVE, got %d", got-writes)
	}

	h.setANE("ane-c", testANEC, gce.ANEStateActive)
	h.syncSettled("both new endpoints are ACTIVE")
	h.expectStatus("both new endpoints are ACTIVE", []string{testANEA + "/32", testANEB + "/32", testANEC + "/32"}, metav1.ConditionTrue, readyReason)
}

func TestStatusController_MarksNodeUpdatingWhenPublishingMidFlight(t *testing.T) {
	h := newANEStatusHarness(t)

	h.setANE("ane-a", testANEA, gce.ANEStateActive)
	h.syncSettled("one endpoint, nothing in flight")

	// Ready=True survives into a batch if a status write that was computed
	// before the batch began lands after the spec controller's Updating.
	// What is published mid-flight must not go out under it.
	h.setANE("ane-b", testANEB, gce.ANEStateActive)
	h.setANE("ane-c", testANEC, gce.ANEStateCreating)
	h.syncInFlight("one of two new endpoints is ACTIVE")
	h.expectStatus("one of two new endpoints is ACTIVE", []string{testANEA + "/32", testANEB + "/32"}, metav1.ConditionFalse, nncReasonUpdating)
}

func TestStatusController_UnpublishesANEAsSoonAsItIsNotActive(t *testing.T) {
	h := newANEStatusHarness(t)
	readyReason := string(nncv1.NodeNetworkConfigReadyReason)

	h.setANE("ane-a", testANEA, gce.ANEStateActive)
	h.setANE("ane-b", testANEB, gce.ANEStateActive)
	h.syncSettled("two endpoints, nothing in flight")
	h.expectStatus("two endpoints, nothing in flight", []string{testANEA + "/32", testANEB + "/32"}, metav1.ConditionTrue, readyReason)

	// Give what is published a recognisable age.
	publishedAt := metav1.NewTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	h.updateStatus(func(live *nncv1.NodeNetworkConfig) {
		for i := range live.Status.PodCIDRs {
			live.Status.PodCIDRs[i].Condition.LastTransitionTime = publishedAt
		}
	})

	// The spec controller is deleting ane-b at the node daemon's request.
	// Status lists what is ACTIVE, so the range goes as soon as its endpoint
	// is DELETING rather than once GCE has finished. The daemon has stopped
	// using it by then, and only waits for the entry to disappear.
	h.setReady(metav1.ConditionFalse, nncReasonUpdating, "Updating GCE VM IP alias ranges")
	h.setANE("ane-b", testANEB, gce.ANEStateDeleting)
	h.syncInFlight("an endpoint is being deleted")
	h.expectStatus("an endpoint is being deleted", []string{testANEA + "/32"}, metav1.ConditionFalse, nncReasonUpdating)
	if kept := h.get().Status.PodCIDRs[0]; !kept.Condition.LastTransitionTime.Equal(&publishedAt) {
		t.Errorf("PodCIDR %q was not affected by the deletion, but its LastTransitionTime moved to %v", kept.CIDR, kept.Condition.LastTransitionTime)
	}

	// The deletion completes. There is no range left to write, but the node
	// is no longer updating and has to be reported ready again.
	h.removeANE("ane-b")
	h.syncSettled("the deletion has completed")
	h.expectStatus("the deletion has completed", []string{testANEA + "/32"}, metav1.ConditionTrue, readyReason)
}

func TestStatusController_ClearsUpdatingOnceNodeHasSettled(t *testing.T) {
	h := newANEStatusHarness(t)

	h.setANE("ane-a", testANEA, gce.ANEStateActive)
	h.syncSettled("one endpoint, nothing in flight")

	// Every range of a batch can already have been published by the time
	// the node settles. The sync that finds it settled then has no range to
	// write, and must report the node ready all the same.
	h.setReady(metav1.ConditionFalse, nncReasonUpdating, "Updating GCE VM IP alias ranges")
	h.syncSettled("node marked Updating with everything published")
	h.expectStatus("node marked Updating with everything published", []string{testANEA + "/32"}, metav1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason))

	// An error reported by the spec controller is a different matter: GCE
	// being quiet and status being up to date does not make it go away.
	failed := string(nncv1.NodeNetworkConfigInvalidParametersReason)
	h.setReady(metav1.ConditionFalse, failed, "GCE mutation failed: quota exceeded")
	writes := h.statusWrites()
	h.syncSettled("node marked failed with everything published")
	if got := h.statusWrites(); got != writes {
		t.Errorf("expected no status write for a node whose only difference is a reported error, got %d", got-writes)
	}
	h.expectStatus("node marked failed with everything published", []string{testANEA + "/32"}, metav1.ConditionFalse, failed)
}

func TestStatusController_KeepsReportedErrorWhilePublishingMidFlight(t *testing.T) {
	h := newANEStatusHarness(t)

	h.setANE("ane-a", testANEA, gce.ANEStateActive)
	h.syncSettled("one endpoint, nothing in flight")

	// A mutation failed part-way: the spec controller has reported the
	// error, and operations it had started are still running in GCE. The
	// endpoint that made it is published; the error is not overwritten.
	failed := string(nncv1.NodeNetworkConfigInvalidParametersReason)
	h.setReady(metav1.ConditionFalse, failed, "GCE mutation failed: quota exceeded")
	h.setANE("ane-b", testANEB, gce.ANEStateActive)
	h.setANE("ane-c", testANEC, gce.ANEStateCreating)

	h.syncInFlight("failed mutation with operations still running")
	h.expectStatus("failed mutation with operations still running", []string{testANEA + "/32", testANEB + "/32"}, metav1.ConditionFalse, failed)
}
