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

package app

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	core "k8s.io/api/core/v1"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
)

const (
	InstanceIDAnnotationKey           = "container.googleapis.com/instance_id"
	NodeTerminationTaintAnnotationKey = "cloud.google.com/impending-node-termination"
)

var (
	errNoMetadata        = fmt.Errorf("instance did not have 'kube-labels' metadata")
	NodeTerminationTaint = &core.Taint{
		Key:    NodeTerminationTaintAnnotationKey,
		Effect: core.TaintEffectNoSchedule,
	}
)

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
	}

	na := &nodeAnnotator{
		c:         client,
		ns:        nodeInformer.Lister(),
		hasSynced: nodeInformer.Informer().HasSynced,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), "node-annotator"),
		//		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node-annotator"),
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
					if instance == nil {
						return false
					}
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
				// Annotator that turns a specific annotation into a taint for the purposes of handling node terminations gracefully.
				name: "node-termination-taint-reconciler",
				annotate: func(node *core.Node, _ *compute.Instance) bool {
					return handleNodeTerminations(node)
				},
			},
			{
				name: "labels-reconciler",
				annotate: func(node *core.Node, instance *compute.Instance) bool {
					if instance == nil {
						return false
					}
					labels, err := extractKubeLabels(instance)
					if err != nil {
						if err != errNoMetadata {
							glog.Errorf("Error reconciling labels: %v", err)
						}
						return false
					}

					if node.ObjectMeta.Labels == nil {
						node.ObjectMeta.Labels = make(map[string]string)
					}

					for _, key := range ownedKubeLabels {
						delete(node.ObjectMeta.Labels, key)
					}

					for key, value := range labels {
						node.ObjectMeta.Labels[key] = value
					}

					return true
				},
			},
		},
	}
	nodeInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				na.enqueue(obj.(*core.Node))
			},
			UpdateFunc: func(old, new interface{}) {
				na.enqueue(old.(*core.Node))
			},
		})
	return na, nil
}

func (na *nodeAnnotator) enqueue(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
		return
	}
	na.queue.Add(key)
}

func (na *nodeAnnotator) Run(workers int, stopCh <-chan struct{}) {
	if !controller.WaitForCacheSync("node-annotator", stopCh, na.hasSynced) {
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

	na.sync(key.(string))
	na.queue.Forget(key)

	return true
}

func (na *nodeAnnotator) work() {
	for na.processNextWorkItem() {
	}
}

func (na *nodeAnnotator) sync(key string) {
	node, err := na.ns.Get(key)
	if err != nil {
		glog.Errorf("Sync %v failed with: %v", key, err)
		na.queue.Add(key)
		return
	}

	instance, err := na.getInstance(node.Spec.ProviderID)
	if err != nil {
		glog.Errorf("Sync %v failed with: %v", key, err)
		na.queue.Add(key)
		return
	}

	var update bool
	for _, ann := range na.annotators {
		modified := ann.annotate(node, instance)
		if modified {
			glog.Infof("%q annotater acting on %q", ann.name, node.Name)
		}
		update = update || modified
	}
	if !update {
		return
	}

	if _, err := na.c.Core().Nodes().Update(node); err != nil {
		glog.Errorf("Sync %v failed with: %v", key, err)
		na.queue.Add(key)
		return
	}
}

type annotator struct {
	name     string
	annotate func(*core.Node, *compute.Instance) bool
}

func parseNodeURL(nodeURL string) (project, zone, instance string, err error) {
	u, err := url.Parse(nodeURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse %q: %v", nodeURL, err)
	}
	if u.Scheme != "gce" {
		return "", "", "", fmt.Errorf("instance %q doesn't run on gce", nodeURL)
	}
	project = u.Host
	parts := strings.Split(u.Path, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("failed to parse %q: expected a three part path", u.Path)
	}
	if len(parts[0]) != 0 {
		return "", "", "", fmt.Errorf("failed to parse %q: part one of path to have length 0", parts[0])
	}
	zone = parts[1]
	instance = parts[2]
	return
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

	labels := make(map[string]string)
	for _, kv := range strings.Split(*kubeLabels, ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("instance %q had malformed label pair: %q", instance.SelfLink, kv)
		}
		labels[parts[0]] = parts[1]
	}
	if err := unversionedvalidation.ValidateLabels(labels, field.NewPath("labels")); len(err) != 0 {
		return nil, fmt.Errorf("instance %q had invalid label(s): %v", instance.SelfLink, err)
	}

	return labels, nil
}

// addOrUpdateTaint tries to add a taint to taint list. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func addOrUpdateTaint(node *core.Node, taint *core.Taint) bool {
	nodeTaints := node.Spec.Taints

	var newTaints []core.Taint
	updated := false
	for i := range nodeTaints {
		if taint.MatchTaint(&nodeTaints[i]) {
			if helper.Semantic.DeepEqual(*taint, nodeTaints[i]) {
				return false
			}
			newTaints = append(newTaints, *taint)
			updated = true
			continue
		}

		newTaints = append(newTaints, nodeTaints[i])
	}

	if !updated {
		newTaints = append(newTaints, *taint)
	}

	node.Spec.Taints = newTaints
	return true
}

func removeTaint(node *core.Node, taintToDelete *core.Taint) bool {
	nodeTaints := node.Spec.Taints

	if !taintExists(nodeTaints, taintToDelete) {
		return false
	}

	// newTaints, _ := deleteTaint(nodeTaints, taint)
	newTaints := []core.Taint{}
	deleted := false
	for _, taint := range nodeTaints {
		if taint.MatchTaint(taintToDelete) {
			deleted = true
			continue
		}
		newTaints = append(newTaints, taint)
	}

	node.Spec.Taints = newTaints
	return deleted
}

func taintExists(taints []core.Taint, taintToFind *core.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

func handleNodeTerminations(node *core.Node) bool {
	glog.V(10).Infof("Checking for possible node termination annotations on node %q: %v", node.Name, node.Spec)
	if len(node.ObjectMeta.Annotations) == 0 {
		// node restarted but no update of ExternalID required
		return removeTaint(node, NodeTerminationTaint)
	}
	value, ok := node.ObjectMeta.Annotations[NodeTerminationTaintAnnotationKey]
	if !ok {
		glog.Infof("Node termination annotation does not exist for node %q. Removing corresponding taint if it exists", node.Name)
		return removeTaint(node, NodeTerminationTaint)
	}
	if value == "true" {
		glog.Infof("Node termination annotation is true for node %q. Tainting the node", node.Name)
		// Need to set node termination taint on the node.
		return addOrUpdateTaint(node, NodeTerminationTaint)
	}
	glog.V(2).Infof("Node termination annotation is false for node %q. Removing corresponding taint if it exists", node.Name)
	return removeTaint(node, NodeTerminationTaint)
}
