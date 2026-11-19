//go:build !providerless
// +build !providerless

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

package gce

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	compute "google.golang.org/api/compute/v1"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func persistDesiredForwardingRuleLabels(t *testing.T, g *Cloud, svc *v1.Service) {
	t.Helper()
	patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{"annotations": map[string]string{ServiceAnnotationLoadBalancerForwardingRuleLabels: svc.Annotations[ServiceAnnotationLoadBalancerForwardingRuleLabels]}}})
	require.NoError(t, err)
	_, err = g.client.CoreV1().Services(svc.Namespace).Patch(context.TODO(), svc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	require.NoError(t, err)
}

func TestMergeForwardingRuleLabels(t *testing.T) {
	for _, tc := range []struct {
		name              string
		existing, desired map[string]string
		previous          string
		want              map[string]string
	}{
		{"preserve unrelated", map[string]string{"external": "value"}, map[string]string{"owned": "new"}, "", map[string]string{"external": "value", "owned": "new"}},
		{"claim conflicting key", map[string]string{"owned": "old"}, map[string]string{"owned": "new"}, "", map[string]string{"owned": "new"}},
		{"withdraw owned key", map[string]string{"owned": "old", "external": "value"}, nil, "owned", map[string]string{"external": "value"}},
		{"empty values are valid", nil, map[string]string{"owned": ""}, "", map[string]string{"owned": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, mergeForwardingRuleLabels(tc.existing, tc.desired, tc.previous))
		})
	}
}

func TestForwardingRuleLabelsDisabled(t *testing.T) {
	g := &Cloud{}
	svc := &v1.Service{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{ServiceAnnotationLoadBalancerForwardingRuleLabels: "malformed", ServiceAnnotationManagedForwardingRuleLabelKeys: "old"}}}
	labels, present, err := g.forwardingRuleLabels(svc)
	require.NoError(t, err)
	require.False(t, present)
	require.Nil(t, labels)
}

func TestReconcileForwardingRuleLabelOwnership(t *testing.T) {
	g, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	g.SetEnableForwardingRuleLabels(true)
	svc := fakeLoadbalancerService("")
	svc.Annotations[ServiceAnnotationLoadBalancerForwardingRuleLabels] = "owned=new"
	svc, err = g.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	rule := &compute.ForwardingRule{Name: "test", LabelFingerprint: "fp", Labels: map[string]string{"external": "keep", "owned": "original"}}
	failCloud := true
	calls := 0
	g.c.(*cloud.MockGCE).MockForwardingRules.SetLabelsHook = func(_ context.Context, _ *meta.Key, request *compute.RegionSetLabelsRequest, _ *cloud.MockForwardingRules, _ ...cloud.Option) error {
		calls++
		stored, err := g.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "owned", stored.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys], "ownership must be persisted before cloud writes")
		if failCloud {
			return fmt.Errorf("cloud write failed")
		}
		rule.Labels = request.Labels
		return nil
	}
	require.ErrorContains(t, g.reconcileForwardingRuleLabels(svc, rule, map[string]string{"owned": "new"}), "cloud write failed")
	require.NotContains(t, svc.Annotations, ServiceAnnotationManagedForwardingRuleLabelKeys, "caller/informer object must not be mutated")
	// A failed write must not lose ownership; retries preserve unrelated labels.
	failCloud = false
	require.NoError(t, g.reconcileForwardingRuleLabels(svc, rule, map[string]string{"owned": "new"}))
	require.Equal(t, map[string]string{"external": "keep", "owned": "new"}, rule.Labels)
	// Removing the desired annotation cleans up only owned keys.
	_, err = g.client.CoreV1().Services(svc.Namespace).Patch(context.TODO(), svc.Name, types.MergePatchType, []byte(`{"metadata":{"annotations":{"cloud.google.com/load-balancer-forwarding-rule-labels":null}}}`), metav1.PatchOptions{})
	require.NoError(t, err)
	svc, err = g.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	desired, present, err := g.forwardingRuleLabels(svc)
	require.NoError(t, err)
	require.True(t, present)
	require.NoError(t, g.reconcileForwardingRuleLabels(svc, rule, desired))
	require.Equal(t, map[string]string{"external": "keep"}, rule.Labels)
	stored, err := g.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotContains(t, stored.Annotations, ServiceAnnotationManagedForwardingRuleLabelKeys)
	require.Equal(t, 3, calls)
}

func TestForwardingRuleLabelOwnershipPatchFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprintf("patch-%d", failAt), func(t *testing.T) {
			g, err := fakeGCECloud(DefaultTestClusterValues())
			require.NoError(t, err)
			svc := fakeLoadbalancerService("")
			svc.Annotations[ServiceAnnotationLoadBalancerForwardingRuleLabels] = "new=value"
			svc.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys] = "old"
			svc, err = g.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)
			patches := 0
			g.client.(*kubefake.Clientset).PrependReactor("patch", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
				patches++
				if patches == failAt {
					return true, nil, fmt.Errorf("patch failed")
				}
				return false, nil, nil
			})
			cloudCalls := 0
			rule := &compute.ForwardingRule{Name: "test", Labels: map[string]string{"old": "value", "external": "keep"}}
			g.c.(*cloud.MockGCE).MockForwardingRules.SetLabelsHook = func(_ context.Context, _ *meta.Key, request *compute.RegionSetLabelsRequest, _ *cloud.MockForwardingRules, _ ...cloud.Option) error {
				cloudCalls++
				rule.Labels = request.Labels
				return nil
			}
			require.ErrorContains(t, g.reconcileForwardingRuleLabels(svc, rule, map[string]string{"new": "value"}), "patch failed")
			stored, err := g.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
			require.NoError(t, err)
			if failAt == 1 {
				require.Zero(t, cloudCalls)
				require.Equal(t, "old", stored.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys])
			} else {
				require.Equal(t, 1, cloudCalls)
				require.Equal(t, "new,old", stored.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys])
			}
			// Retrying after either failure converges and then drops withdrawn keys.
			require.NoError(t, g.reconcileForwardingRuleLabels(stored, rule, map[string]string{"new": "value"}))
			require.Equal(t, map[string]string{"external": "keep", "new": "value"}, rule.Labels)
			stored, err = g.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, "new", stored.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys])
		})
	}
}
