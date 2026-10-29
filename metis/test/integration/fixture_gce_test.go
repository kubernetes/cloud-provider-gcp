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
	"fmt"

	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computebeta "google.golang.org/api/compute/v0.beta"
	computepreview "google.golang.org/api/compute/v1.preview"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// resolveAliasRanges translates netmask-only placeholders (e.g. "/28") into
// concrete mock IP ranges (e.g. "10.100.0.0/28") to simulate GCE IPAM allocation.
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

// gceAliasRanges returns the CIDRs of all alias IP ranges currently attached
// to the fake GCE instance's primary interface.
func (f *fixture) gceAliasRanges() []string {
	f.t.Helper()
	inst, err := f.gce.Compute().BetaInstances().Get(f.ctx, instanceKey)
	if err != nil {
		f.t.Fatalf("failed to get fake GCE instance: %v", err)
	}
	for _, iface := range inst.NetworkInterfaces {
		if iface.Name == "nic0" {
			var ranges []string
			for _, r := range iface.AliasIpRanges {
				ranges = append(ranges, r.IpCidrRange)
			}
			return ranges
		}
	}
	return nil
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
