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
	"sort"
	"strings"
	"testing"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computebeta "google.golang.org/api/compute/v0.beta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

// These tests cover how the status controller reacts when an alias IP range
// disappears from the VM without having been requested through
// spec.releasableCIDRs -- the "out of band" case (gcloud, Terraform, a GCE-side
// operation). The status controller is the only component that re-reads GCE,
// so it is the one that has to (a) make the spec controller re-apply
// spec.allocations against the reduced capacity and (b) tell an operator that
// pods on this node just lost a range.

// recordingSpecTrigger captures every EnqueueNode call.
type recordingSpecTrigger struct {
	calls []string
}

func (r *recordingSpecTrigger) EnqueueNode(nodeName string) {
	r.calls = append(r.calls, nodeName)
}

// driftFixture is a node whose NodeNetworkConfig asks for more pods than one
// range provides, backed by an instance that already holds enough ranges to
// satisfy it.
type driftFixture struct {
	*testFixture
	ctx         context.Context
	instanceKey *meta.Key
}

// newDriftFixture seeds an instance holding cidrs, a NodeNetworkConfig
// requesting 16 pods per range, and a Node carrying the provider ID. The
// informers are deliberately not started so that the only items in either
// controller's queue are the ones the code under test put there.
func newDriftFixture(t *testing.T, releasable []string, cidrs ...string) *driftFixture {
	t.Helper()
	ctx := context.Background()
	f := newTestFixture(t)

	instanceKey := insertInstanceWithRanges(ctx, t, f, cidrs...)

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: int32(16 * len(cidrs))}},
		},
	}
	for _, cidr := range releasable {
		nnc.Spec.ReleasableCIDRs = append(nnc.Spec.ReleasableCIDRs, nncv1.PodCIDR{Id: cidr, Network: "default", CIDR: cidr})
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	if _, err := f.kubeClient.CoreV1().Nodes().Create(ctx, nodeWithProviderID(testProviderID), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Node: %v", err)
	}

	return &driftFixture{testFixture: f, ctx: ctx, instanceKey: instanceKey}
}

// removeRangeOutOfBand drops cidr from the mock instance directly, bypassing
// the provider, exactly as an operator or another system would.
func (d *driftFixture) removeRangeOutOfBand(cidr string) {
	d.t.Helper()
	mockInstances, ok := d.fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		d.t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	instObj, err := mockInstances.Get(d.ctx, d.instanceKey)
	if err != nil {
		d.t.Fatalf("Failed to get instance from mock: %v", err)
	}
	var kept []*computebeta.AliasIpRange
	for _, r := range instObj.NetworkInterfaces[0].AliasIpRanges {
		if r.IpCidrRange != cidr {
			kept = append(kept, r)
		}
	}
	if len(kept) == len(instObj.NetworkInterfaces[0].AliasIpRanges) {
		d.t.Fatalf("Range %q was not attached to the instance", cidr)
	}
	mockInstances.Lock.Lock()
	instObj.NetworkInterfaces[0].AliasIpRanges = kept
	mockInstances.Objects[*d.instanceKey] = &gcloud.MockInstancesObj{Obj: instObj}
	mockInstances.Lock.Unlock()
}

// gceRanges returns the alias IP ranges currently attached to the instance.
func (d *driftFixture) gceRanges() []string {
	d.t.Helper()
	inst, err := d.fakeGCE.Compute().BetaInstances().Get(d.ctx, d.instanceKey)
	if err != nil {
		d.t.Fatalf("Failed to get GCE instance: %v", err)
	}
	var got []string
	for _, r := range inst.NetworkInterfaces[0].AliasIpRanges {
		got = append(got, r.IpCidrRange)
	}
	sort.Strings(got)
	return got
}

// nncObject returns the current NodeNetworkConfig.
func (d *driftFixture) nncObject() *nncv1.NodeNetworkConfig {
	d.t.Helper()
	nnc, err := d.nncClient.NetworkingV1().NodeNetworkConfigs().Get(d.ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		d.t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	return nnc
}

// statusRanges returns the CIDRs currently published in Status.PodCIDRs.
func (d *driftFixture) statusRanges() []string {
	d.t.Helper()
	var got []string
	for _, pc := range d.nncObject().Status.PodCIDRs {
		got = append(got, pc.CIDR)
	}
	sort.Strings(got)
	return got
}

func (d *driftFixture) syncStatus() {
	d.t.Helper()
	if err := d.statusCtrl.syncNode(testNodeName); err != nil {
		d.t.Fatalf("Status sync failed: %v", err)
	}
}

// processOne runs a single work item from queue, failing fast rather than
// blocking forever if nothing was enqueued.
func processOne(t *testing.T, name string, queue workqueue.TypedRateLimitingInterface[string], process func() bool) {
	t.Helper()
	if queue.Len() == 0 {
		t.Fatalf("Expected %s queue to hold an item, but it is empty", name)
	}
	if !process() {
		t.Fatalf("%s queue unexpectedly shut down", name)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestStatusController_TriggersSpecOnlyOnChangedPublish pins the contract the
// spec controller depends on: it is poked exactly when status was rewritten,
// which is exactly when GCE no longer matches what was last published.
func TestStatusController_TriggersSpecOnlyOnChangedPublish(t *testing.T) {
	d := newDriftFixture(t, nil, "10.100.0.0/28", "10.100.1.0/28")
	trigger := &recordingSpecTrigger{}
	d.statusCtrl.SetSpecTrigger(trigger)

	// First publish: empty status -> two ranges.
	d.syncStatus()
	if got := d.statusRanges(); !equalStrings(got, []string{"10.100.0.0/28", "10.100.1.0/28"}) {
		t.Fatalf("Expected both ranges published, got %v", got)
	}
	if got := trigger.calls; !equalStrings(got, []string{testNodeName}) {
		t.Fatalf("Expected one spec trigger after the first publish, got %v", got)
	}

	// Nothing changed: a sync must not cost the spec controller a reconcile.
	d.syncStatus()
	if got := len(trigger.calls); got != 1 {
		t.Fatalf("Expected no spec trigger for an unchanged status, got %d calls", got)
	}

	// A range vanishes out of band: status shrinks, spec must be re-evaluated.
	d.removeRangeOutOfBand("10.100.0.0/28")
	d.syncStatus()
	if got := d.statusRanges(); !equalStrings(got, []string{"10.100.1.0/28"}) {
		t.Fatalf("Expected only the surviving range published, got %v", got)
	}
	if got := len(trigger.calls); got != 2 {
		t.Fatalf("Expected a second spec trigger after out-of-band removal, got %d calls", got)
	}
}

// TestStatusController_EventOnOutOfBandRemoval covers the operator-facing side:
// a removal nobody asked for is a Warning on the Node, a removal the daemon
// requested through spec.releasableCIDRs is routine and must stay silent.
func TestStatusController_EventOnOutOfBandRemoval(t *testing.T) {
	const (
		requested   = "10.100.0.0/28"
		unrequested = "10.100.1.0/28"
		untouched   = "10.100.2.0/28"
	)
	d := newDriftFixture(t, []string{requested}, requested, unrequested, untouched)

	drainEvents := func() []string {
		var events []string
		for {
			select {
			case e := <-d.events.Events:
				events = append(events, e)
			default:
				return events
			}
		}
	}

	// Initial publish is not a removal of anything.
	d.syncStatus()
	if got := drainEvents(); len(got) != 0 {
		t.Fatalf("Expected no events on first publish, got %v", got)
	}

	// The daemon asked for this one: the spec controller would have removed it.
	d.removeRangeOutOfBand(requested)
	d.syncStatus()
	if got := drainEvents(); len(got) != 0 {
		t.Fatalf("Expected no event for a range listed in spec.releasableCIDRs, got %v", got)
	}

	// Nobody asked for this one.
	d.removeRangeOutOfBand(unrequested)
	d.syncStatus()
	got := drainEvents()
	if len(got) != 1 {
		t.Fatalf("Expected exactly one event for an out-of-band removal, got %v", got)
	}
	for _, want := range []string{corev1.EventTypeWarning, PodCIDRRemovedOutOfBandReason, unrequested, `"default"`} {
		if !strings.Contains(got[0], want) {
			t.Errorf("Event %q does not mention %q", got[0], want)
		}
	}
	if strings.Contains(got[0], untouched) {
		t.Errorf("Event %q names a range that is still attached", got[0])
	}

	// Re-syncing with nothing new must not repeat the warning.
	d.syncStatus()
	if got := drainEvents(); len(got) != 0 {
		t.Fatalf("Expected no event on an unchanged re-sync, got %v", got)
	}

	// Events are recorded against the Node object so they carry its UID. If the
	// Node cannot be resolved the event is skipped; the status write itself must
	// not be affected.
	if err := d.kubeClient.CoreV1().Nodes().Delete(d.ctx, testNodeName, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete Node: %v", err)
	}
	d.removeRangeOutOfBand(untouched)
	if err := d.statusCtrl.reconcile(d.ctx, d.nncObject(), testProviderID); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if got := d.statusRanges(); len(got) != 0 {
		t.Fatalf("Expected status to be emptied even without a Node to record against, got %v", got)
	}
	if got := drainEvents(); len(got) != 0 {
		t.Fatalf("Expected no event when the Node cannot be resolved, got %v", got)
	}
}

// TestStatusController_SpecTriggerRestoresCapacity wires the two real
// controllers together and walks the full loop by hand. Before the trigger
// existed, a node whose spec.allocations already exceeded the VM's capacity
// after an out-of-band removal stayed that way: the spec controller only
// reconciles on spec generation changes, and a daemon whose ask is already
// above capacity has no reason to change it.
func TestStatusController_SpecTriggerRestoresCapacity(t *testing.T) {
	const (
		lost      = "10.100.5.0/28"
		surviving = "10.100.6.0/28"
	)
	d := newDriftFixture(t, nil, lost, surviving)
	d.statusCtrl.SetSpecTrigger(d.specCtrl)

	specQueue, statusQueue := d.specCtrl.queue, d.statusCtrl.queue
	runSpec := func() { processOne(t, "spec", specQueue, d.specCtrl.processNextWorkItem) }
	runStatus := func() { processOne(t, "status", statusQueue, d.statusCtrl.processNextWorkItem) }

	// Steady state: status published, spec (32 pods) satisfied by GCE (2 ranges).
	d.syncStatus()
	runSpec()
	if got := d.gceRanges(); !equalStrings(got, []string{lost, surviving}) {
		t.Fatalf("Expected GCE untouched while spec is satisfied, got %v", got)
	}
	runStatus() // the spec controller always hands the node back; nothing changed, so this is a no-op
	if got := specQueue.Len(); got != 0 {
		t.Fatalf("Expected an unchanged status not to re-trigger the spec controller, queue length %d", got)
	}

	// A range is deleted behind everyone's back. Spec still says 32 pods.
	d.removeRangeOutOfBand(lost)
	d.syncStatus()
	if got := d.statusRanges(); !equalStrings(got, []string{surviving}) {
		t.Fatalf("Expected status to reflect the loss, got %v", got)
	}

	// The changed publish is what re-runs the spec controller, which now sees
	// 32 requested against 16 present and attaches a fresh range.
	runSpec()
	got := d.gceRanges()
	if len(got) != 2 {
		t.Fatalf("Expected capacity restored to 2 ranges, got %v", got)
	}
	for _, cidr := range got {
		if cidr == lost {
			t.Fatalf("Expected a fresh range rather than the deleted one reappearing, got %v", got)
		}
	}

	// And the loop converges: spec -> status publishes the new range -> spec
	// finds nothing to do -> status finds nothing to publish.
	runStatus()
	if got := d.statusRanges(); !equalStrings(got, d.gceRanges()) {
		t.Fatalf("Expected status to match GCE after restoration, status=%v gce=%v", got, d.gceRanges())
	}
	runSpec()
	runStatus()
	if got := specQueue.Len(); got != 0 {
		t.Fatalf("Expected the controllers to settle, but the spec queue still holds %d item(s)", got)
	}
	if got := statusQueue.Len(); got != 0 {
		t.Fatalf("Expected the controllers to settle, but the status queue still holds %d item(s)", got)
	}
}
