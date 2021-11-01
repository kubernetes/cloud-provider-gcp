/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	compute "google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	taintsutil "k8s.io/kubernetes/pkg/util/taints"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	// InstanceIDAnnotationKey is the node annotation key where the external ID is written.
	InstanceIDAnnotationKey = "container.googleapis.com/instance_id"
	lastAppliedLabelsKey    = "node.gke.io/last-applied-node-labels"
	lastAppliedTaintsKey    = "node.gke.io/last-applied-node-taints"
)

var errNoMetadata = fmt.Errorf("instance did not have 'kube-labels' metadata")

type nodeAnnotator struct {
	c          clientset.Interface
	ns         corelisters.NodeLister
	hasSynced  func() bool
	queue      workqueue.RateLimitingInterface
	annotators []annotator
	// for testing
	getInstance func(nodeURL string) (*compute.Instance, error)
}

func newNodeAnnotator(client clientset.Interface, nodeInformer coreinformers.NodeInformer, cs *compute.Service) (*nodeAnnotator, error) {
	gce := compute.NewInstancesService(cs)

	// TODO(mikedanese): create a registry for the labels that GKE uses. This was
	// lifted from node_startup.go and the naming scheme is adhoc and
	// inconsistent.
	ownedKubeLabels := []string{
		"cloud.google.com/gke-nodepool",
		"cloud.google.com/gke-local-ssd",
		"cloud.google.com/gke-local-scsi-ssd",
		"cloud.google.com/gke-local-nvme-ssd",
		"cloud.google.com/gke-preemptible",
		"cloud.google.com/gke-gpu",
		"cloud.google.com/gke-accelerator",
		"beta.kubernetes.io/fluentd-ds-ready",
		"beta.kubernetes.io/kube-proxy-ds-ready",
		"beta.kubernetes.io/masq-agent-ds-ready",
		"projectcalico.org/ds-ready",
		"beta.kubernetes.io/metadata-proxy-ready",
		"addon.gke.io/node-local-dns-ds-ready",
	}

	na := &nodeAnnotator{
		c:         client,
		ns:        nodeInformer.Lister(),
		hasSynced: nodeInformer.Informer().HasSynced,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), "node-annotator"),
		getInstance: func(nodeURL string) (*compute.Instance, error) {
			project, zone, instance, err := parseNodeURL(nodeURL)
			if err != nil {
				return nil, err
			}
			return gce.Get(project, zone, instance).Do()
		},
		annotators: []annotator{
			{
				name: "instance-id-reconciler",
				annotate: func(node *core.Node, instance *compute.Instance) bool {
					eid := strconv.FormatUint(instance.Id, 10)
					if len(node.ObjectMeta.Annotations) != 0 && eid == node.ObjectMeta.Annotations[InstanceIDAnnotationKey] {
						// node restarted but no update of ExternalID required
						return false
					}
					if node.ObjectMeta.Annotations == nil {
						node.ObjectMeta.Annotations = make(map[string]string)
					}
					node.ObjectMeta.Annotations[InstanceIDAnnotationKey] = eid
					return true
				},
			},
			{
				name: "labels-reconciler",
				annotate: func(node *core.Node, instance *compute.Instance) bool {
					klog.Infof("Triggering label reconcilation")
					desiredLabels, err := extractKubeLabels(instance)
					if err != nil {
						if err != errNoMetadata {
							klog.Errorf("Error reconciling labels: %v", err)
						}
						return false
					}

					if node.ObjectMeta.Labels == nil {
						node.ObjectMeta.Labels = make(map[string]string)
					}

					for _, key := range ownedKubeLabels {
						delete(node.ObjectMeta.Labels, key)
					}

					err = mergeManagedLabels(node, desiredLabels)
					if err != nil {
						klog.Errorf("Error merging labels: %v", err)
						return false
					}

					return true
				},
			},
			{
				name: "taints-reconciler",
				annotate: func(node *core.Node, instance *compute.Instance) bool {
					klog.Infof("Triggering taint reconcilation")
					desiredTaints, err := extractNodeTaints(instance)
					if err != nil {
						if err != errNoMetadata {
							klog.Errorf("Error reconciling taints: %v", err)
						}
						return false
					}

					err = mergeManagedTaints(node, desiredTaints)
					if err != nil {
						klog.Errorf("Error merging taints: %v", err)
						return false
					}

					return true
				},
			},
		},
	}
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    na.add,
		UpdateFunc: na.update,
	})
	return na, nil
}

func (na *nodeAnnotator) add(obj interface{}) {
	na.enqueue(obj)
}

func (na *nodeAnnotator) update(obj, oldObj interface{}) {
	node := obj.(*core.Node)
	oldNode := oldObj.(*core.Node)
	if node.Status.NodeInfo.BootID != oldNode.Status.NodeInfo.BootID {
		na.enqueue(obj)
	}
}

func (na *nodeAnnotator) enqueue(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	na.queue.Add(key)
}

func (na *nodeAnnotator) Run(workers int, stopCh <-chan struct{}) {
	if !cache.WaitForNamedCacheSync("node-annotator", stopCh, na.hasSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(na.work, time.Second, stopCh)
	}
	<-stopCh
}

func (na *nodeAnnotator) processNextWorkItem() bool {
	key, quit := na.queue.Get()
	if quit {
		return false
	}
	defer na.queue.Done(key)

	err := na.sync(key.(string))
	if err != nil {
		klog.Warningf("Requeue %v (%v times) due to err: %v", key, na.queue.NumRequeues(key), err)
		na.queue.AddRateLimited(key)
		return true
	}
	// Item successfully proceeded, remove from rate limiter.
	na.queue.Forget(key)
	return true
}

func (na *nodeAnnotator) work() {
	for na.processNextWorkItem() {
	}
}

func (na *nodeAnnotator) sync(key string) error {
	node, err := na.ns.Get(key)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Node %v doesn't exist, dropping from the queue", key)
			return nil
		}
		return err
	}

	instance, err := na.getInstance(node.Spec.ProviderID)
	if err != nil {
		return err
	}

	var update bool
	for _, ann := range na.annotators {
		modified := ann.annotate(node, instance)
		if modified {
			klog.Infof("%q annotater acting on %q", ann.name, node.Name)
		}
		update = update || modified
	}
	if !update {
		return nil
	}

	_, err = na.c.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

type annotator struct {
	name     string
	annotate func(*core.Node, *compute.Instance) bool
}

func parseNodeURL(nodeURL string) (project, zone, instance string, err error) {
	// We only expect to handle strings that look like:
	// gce://project/zone/instance
	if !strings.HasPrefix(nodeURL, "gce://") {
		return "", "", "", fmt.Errorf("instance %q doesn't run on gce", nodeURL)
	}
	parts := strings.Split(strings.TrimPrefix(nodeURL, "gce://"), "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("failed to parse %q: expected a three part path", nodeURL)
	}
	return parts[0], parts[1], parts[2], nil
}

// TODO: move this to instance.Labels. This is gross.
func extractKubeLabels(instance *compute.Instance) (map[string]string, error) {
	const labelsKey = "kube-labels"

	if instance.Metadata == nil {
		return nil, errNoMetadata
	}

	var kubeLabels *string
	for _, item := range instance.Metadata.Items {
		if item == nil || item.Key != labelsKey {
			continue
		}
		if item.Value == nil {
			return nil, fmt.Errorf("instance %q had nil %q", instance.SelfLink, labelsKey)
		}
		kubeLabels = item.Value
	}
	if kubeLabels == nil {
		return nil, errNoMetadata
	}
	if len(*kubeLabels) == 0 {
		return make(map[string]string), nil
	}

	parsedLabels, err := parseLabels(*kubeLabels)
	if err != nil {
		return nil, fmt.Errorf("instance %q had %s", instance.SelfLink, err.Error())
	}
	return parsedLabels, nil
}

func extractNodeTaints(instance *compute.Instance) ([]core.Taint, error) {
	const kubeEnvKey = "kube-env"

	if instance.Metadata == nil {
		return nil, errNoMetadata
	}

	var kubeEnv *string
	for _, item := range instance.Metadata.Items {
		if item == nil || item.Key != kubeEnvKey {
			continue
		}
		if item.Value == nil {
			return nil, fmt.Errorf("instance %q had nil %q", instance.SelfLink, kubeEnvKey)
		}
		kubeEnv = item.Value
		break
	}
	if kubeEnv == nil {
		return nil, errNoMetadata
	}
	if len(*kubeEnv) == 0 {
		klog.Infof("Node taints not found in instance metadata, %s is empty", kubeEnvKey)
		return nil, nil
	}

	var taintsEnv string
	for _, env := range strings.Split(*kubeEnv, ";") {
		if strings.HasPrefix(env, "node_taints") {
			taintsEnv = env
			break
		}
	}
	if taintsEnv == "" {
		// No taints present in the instance metadata.
		klog.Infof("Node taints not found in instance metadata, node_taints not found in %s", kubeEnvKey)
		return nil, nil
	}
	parts := strings.SplitN(taintsEnv, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed node_taints in instance metadata")
	}
	parsedTaints, err := parseTaints(parts[1])
	if err != nil {
		return nil, err
	}
	return parsedTaints, nil
}

func extractLastAppliedLabels(node *core.Node) map[string]string {
	lastLabels, ok := node.ObjectMeta.Annotations[lastAppliedLabelsKey]
	if !ok || len(lastLabels) == 0 {
		return nil
	}

	parsedLabels, err := parseLabels(lastLabels)
	if err != nil {
		klog.Errorf("Failed to parse last applied labels annotation: %q, treat it as not set, err: %v", lastLabels, err)
		return nil
	}
	return parsedLabels
}

func extractLastAppliedTaints(node *core.Node) []core.Taint {
	lastTaints, ok := node.ObjectMeta.Annotations[lastAppliedTaintsKey]
	if !ok || len(lastTaints) == 0 {
		return nil
	}

	parsedTaints, err := parseTaints(lastTaints)
	if err != nil {
		klog.Errorf("Failed to parse last applied taints annotation: %q, treat it as not set, err: %v", lastTaints, err)
		return nil
	}
	return parsedTaints
}

func mergeManagedLabels(node *core.Node, desiredLabels map[string]string) error {
	if node.ObjectMeta.Annotations == nil {
		node.ObjectMeta.Annotations = make(map[string]string)
	}
	lastAppliedLabels := extractLastAppliedLabels(node)
	// Merge GCE managed labels by:
	// 1. delete managed labels to be removed, which is present in last-applied-labels
	// 2. add/update labels from GCE label source to node
	// 3. update last-applied-labels in annotation
	for key := range lastAppliedLabels {
		delete(node.ObjectMeta.Labels, key)
	}
	for key, value := range desiredLabels {
		node.ObjectMeta.Labels[key] = value
	}
	node.ObjectMeta.Annotations[lastAppliedLabelsKey] = serializeLabels(desiredLabels)
	return nil
}

func mergeManagedTaints(node *core.Node, desiredTaints []core.Taint) error {
	if node.ObjectMeta.Annotations == nil {
		node.ObjectMeta.Annotations = make(map[string]string)
	}
	lastAppliedTaints := extractLastAppliedTaints(node)
	// Merge GCE managed taints by:
	// 1. delete managed taints to be removed, which is present in last-applied-taints
	// 2. add/update taints from GCE taint source to node
	// 3. update last-applied-taints in annotation
	for _, taint := range lastAppliedTaints {
		node.Spec.Taints, _ = taintsutil.DeleteTaint(node.Spec.Taints, &taint)
	}
	for _, taint := range desiredTaints {
		updated := false
		for i := range node.Spec.Taints {
			if taint.MatchTaint(&node.Spec.Taints[i]) {
				node.Spec.Taints[i] = taint
				updated = true
			}
		}
		if !updated {
			node.Spec.Taints = append(node.Spec.Taints, taint)
		}
	}
	node.ObjectMeta.Annotations[lastAppliedTaintsKey] = serializeTaints(desiredTaints)
	return nil
}

func parseLabels(labelString string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, kv := range strings.Split(labelString, ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed label pair: %q", kv)
		}
		labels[parts[0]] = parts[1]
	}
	if err := unversionedvalidation.ValidateLabels(labels, field.NewPath("labels")); len(err) != 0 {
		return nil, fmt.Errorf("invalid label(s): %v", err)
	}
	return labels, nil
}

func parseTaints(taintsString string) ([]core.Taint, error) {
	var taints []core.Taint
	taintsList := strings.Split(taintsString, ",")
	taints, _, err := taintsutil.ParseTaints(taintsList)
	if err != nil {
		return nil, err
	}
	return taints, nil
}

func serializeLabels(labels map[string]string) string {
	labelElements := make([]string, 0, len(labels))
	for key, value := range labels {
		labelElements = append(labelElements, fmt.Sprintf("%s=%s", key, value))
	}
	// Sort labels to avoid test flakes.
	sort.Strings(labelElements)
	return strings.Join(labelElements, ",")
}

func serializeTaints(taints []core.Taint) string {
	taintElements := make([]string, 0, len(taints))
	for _, taint := range taints {
		taintElements = append(taintElements, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, string(taint.Effect)))
	}
	// Sort taints to avoid test flakes.
	sort.Strings(taintElements)
	return strings.Join(taintElements, ",")
}
