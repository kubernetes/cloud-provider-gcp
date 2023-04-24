/*
Copyright 2022 The Kubernetes Authors.

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

package gkenetworkparamset

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/compute/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	networkv1alpha1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1alpha1"
	networkclientset "k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned"
	"k8s.io/cloud-provider-gcp/providers/gce"

	controllersmetrics "k8s.io/component-base/metrics/prometheus/controllers"
)

const (
	// GNPFinalizer - finalizer value placed on GNP objects by GNP Controller
	GNPFinalizer = "networking.gke.io/gnp-controller"
)

// Controller manages GKENetworkParamSet status.
type Controller struct {
	gkeNetworkParamsInformer cache.SharedIndexInformer
	networkClientset         networkclientset.Interface
	gceCloud                 *gce.Cloud
	queue                    workqueue.RateLimitingInterface
}

// NewGKENetworkParamSetController returns a new
func NewGKENetworkParamSetController(
	networkClientset networkclientset.Interface,
	gkeNetworkParamsInformer cache.SharedIndexInformer,
	gceCloud *gce.Cloud,
) *Controller {

	// register GNP metrics
	registerGKENetworkParamSetMetrics()

	return &Controller{
		networkClientset:         networkClientset,
		gkeNetworkParamsInformer: gkeNetworkParamsInformer,
		gceCloud:                 gceCloud,
		queue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "gkenetworkparamset"),
	}

}

// Run starts an asynchronous loop that monitors and updates GKENetworkParamSet in the cluster.
func (c *Controller) Run(numWorkers int, stopCh <-chan struct{}, controllerManagerMetrics *controllersmetrics.ControllerManagerMetrics) {
	defer utilruntime.HandleCrash()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	defer c.queue.ShutDown()

	klog.Infof("Starting gkenetworkparamset controller")
	defer klog.Infof("Shutting down gkenetworkparamset controller")
	controllerManagerMetrics.ControllerStarted("gkenetworkparamset")
	defer controllerManagerMetrics.ControllerStopped("gkenetworkparamset")

	c.gkeNetworkParamsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(key)
			}
		},
	})

	if !cache.WaitForNamedCacheSync("gkenetworkparamset", stopCh, c.gkeNetworkParamsInformer.HasSynced) {
		return
	}
	for i := 0; i < numWorkers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-stopCh
}

// worker pattern adapted from https://github.com/kubernetes/client-go/blob/master/examples/workqueue/main.go
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *Controller) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)

	err := c.syncGKENetworkParamSet(ctx, key.(string))
	c.handleErr(err, key)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		klog.Warningf("Error while updating GKENetworkParamSet object, retrying %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	utilruntime.HandleError(err)
	klog.Errorf("Dropping GKENetworkParamSet %q out of the queue: %v", key, err)
}

// addFinalizerToGKENetworkParamSet adds a finalizer to params inplace if it doesnt already exist
func addFinalizerToGKENetworkParamSet(params *networkv1alpha1.GKENetworkParamSet) {
	gnpFinalizers := params.GetFinalizers()
	for _, f := range gnpFinalizers {
		if f == GNPFinalizer {
			return
		}
	}

	params.SetFinalizers(append(gnpFinalizers, GNPFinalizer))
}

func (c *Controller) syncGKENetworkParamSet(ctx context.Context, key string) error {
	obj, exists, err := c.gkeNetworkParamsInformer.GetIndexer().GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		// GKENetworkParamSet does not exist anymore since the work was queued, so move on
		return nil
	}

	params := obj.(*networkv1alpha1.GKENetworkParamSet)

	// TODO: Enable finalizer addition when finalizer deletion is added.
	// addFinalizerToGKENetworkParamSet(params)
	// update will be done once in deferred function call
	// if err := c.updateGKENetworkParamSet(ctx, params); err != nil {
	// 	return err
	// }

	subnet, err := c.gceCloud.GetSubnetwork(c.gceCloud.Region(), params.Spec.VPCSubnet)
	if err != nil {
		fetchSubnetErrs.Inc()
		return err
	}

	cidrs := extractRelevantCidrs(subnet, params)

	err = c.updateGKENetworkParamSetStatus(ctx, params, cidrs)
	if err != nil {
		return err
	}

	return nil
}

// extractRelevantCidrs returns the CIDRS of the named ranges in paramset
func extractRelevantCidrs(subnet *compute.Subnetwork, paramset *networkv1alpha1.GKENetworkParamSet) []string {
	cidrs := []string{}

	// use the subnet cidr if there are no secondary ranges specified by user in params
	if paramset.Spec.PodIPv4Ranges == nil || (paramset.Spec.PodIPv4Ranges != nil && len(paramset.Spec.PodIPv4Ranges.RangeNames) == 0) {
		cidrs = append(cidrs, subnet.IpCidrRange)
		return cidrs
	}

	// get secondary ranges' cooresponding cidrs
	for _, sr := range subnet.SecondaryIpRanges {
		if !paramSetIncludesRange(paramset, sr.RangeName) {
			continue
		}

		cidrs = append(cidrs, sr.IpCidrRange)
	}
	return cidrs
}

func paramSetIncludesRange(params *networkv1alpha1.GKENetworkParamSet, secondaryRangeName string) bool {
	for _, rn := range params.Spec.PodIPv4Ranges.RangeNames {
		if rn == secondaryRangeName {
			return true
		}
	}
	return false
}

func (c *Controller) updateGKENetworkParamSet(ctx context.Context, params *networkv1alpha1.GKENetworkParamSet) error {
	_, err := c.networkClientset.NetworkingV1alpha1().GKENetworkParamSets().Update(ctx, params, v1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update GKENetworkParamSet: %v", err)
	}
	return nil
}

func (c *Controller) updateGKENetworkParamSetStatus(ctx context.Context, gkeNetworkParamSet *networkv1alpha1.GKENetworkParamSet, cidrs []string) error {
	gkeNetworkParamSet.Status.PodCIDRs = &networkv1alpha1.NetworkRanges{
		CIDRBlocks: cidrs,
	}

	klog.V(4).Infof("GKENetworkParamSet cidrs are: %v", cidrs)
	_, err := c.networkClientset.NetworkingV1alpha1().GKENetworkParamSets().UpdateStatus(ctx, gkeNetworkParamSet, v1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update GKENetworkParamSet Status CIDRs: %v", err)
	}
	return nil
}
