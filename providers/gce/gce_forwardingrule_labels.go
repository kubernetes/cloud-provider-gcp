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
	"maps"
	"sort"
	"strings"

	compute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ServiceAnnotationManagedForwardingRuleLabelKeys is CCM-owned recovery state.
// Other controllers must not modify it when changing desired labels.
const ServiceAnnotationManagedForwardingRuleLabelKeys = "cloud.google.com/managed-load-balancer-forwarding-rule-label-keys"

func (g *Cloud) forwardingRuleLabels(svc *v1.Service) (map[string]string, bool, error) {
	if !g.enableForwardingRuleLabels {
		return nil, false, nil
	}
	labels, present, err := GetLoadBalancerAnnotationForwardingRuleLabels(svc)
	return labels, present || svc.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys] != "", err
}

func labelKeysValue(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func mergeForwardingRuleLabels(existing, desired map[string]string, previous string) map[string]string {
	labels := maps.Clone(existing)
	if labels == nil {
		labels = map[string]string{}
	}
	for _, key := range strings.Split(previous, ",") {
		if key != "" {
			delete(labels, key)
		}
	}
	maps.Copy(labels, desired)
	return labels
}

func (g *Cloud) patchManagedForwardingRuleLabelKeys(svc *v1.Service, value string) error {
	if svc.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys] == value {
		return nil
	}
	var annotationValue interface{} = value
	if value == "" {
		annotationValue = nil
	}
	// Optimistic locking prevents a stale ensure from claiming keys for an obsolete
	// desired annotation. Updating the local resourceVersion also protects the
	// completion patch below from concurrent Service changes.
	patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{
		"resourceVersion": svc.ResourceVersion, "uid": svc.UID,
		"annotations": map[string]interface{}{ServiceAnnotationManagedForwardingRuleLabelKeys: annotationValue},
	}})
	if err != nil {
		return err
	}
	updated, err := g.client.CoreV1().Services(svc.Namespace).Patch(context.TODO(), svc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("persist forwarding-rule label ownership: %w", err)
	}
	svc.ResourceVersion = updated.ResourceVersion
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	if value == "" {
		delete(svc.Annotations, ServiceAnnotationManagedForwardingRuleLabelKeys)
	} else {
		svc.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys] = value
	}
	return nil
}

func (g *Cloud) reconcileForwardingRuleLabels(svc *v1.Service, rule *compute.ForwardingRule, desired map[string]string) error {
	current, err := g.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get Service for label ownership: %w", err)
	}
	oldValue, oldPresent := svc.Annotations[ServiceAnnotationLoadBalancerForwardingRuleLabels]
	newValue, newPresent := current.Annotations[ServiceAnnotationLoadBalancerForwardingRuleLabels]
	if current.UID != svc.UID || oldValue != newValue || oldPresent != newPresent {
		return fmt.Errorf("Service changed during forwarding-rule label reconciliation")
	}
	svc = current
	previous := svc.Annotations[ServiceAnnotationManagedForwardingRuleLabelKeys]
	labels := mergeForwardingRuleLabels(rule.Labels, desired, previous)
	if len(labels) > maxGCELabelsPerResource {
		return fmt.Errorf("merged forwarding-rule labels exceed limit of %d", maxGCELabelsPerResource)
	}
	// Journal the union BEFORE the cloud write. If the write or final patch fails,
	// retries can still remove every potentially applied key, including keys
	// withdrawn by a subsequent Service update.
	pending := mergeForwardingRuleLabels(nil, desired, "")
	for _, key := range strings.Split(previous, ",") {
		if key != "" {
			pending[key] = ""
		}
	}
	if err := g.patchManagedForwardingRuleLabelKeys(svc, labelKeysValue(pending)); err != nil {
		return err
	}
	if err := g.SetRegionForwardingRuleLabels(rule, g.region, labels); err != nil {
		return err
	}
	return g.patchManagedForwardingRuleLabelKeys(svc, labelKeysValue(desired))
}
