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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	networkclientset "k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned"
	networkinformers "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions"
	networkinformer "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1"
	"k8s.io/cloud-provider-gcp/pkg/controllermetrics"
	"k8s.io/cloud-provider-gcp/providers/gce"
	controllersmetrics "k8s.io/component-base/metrics/prometheus/controllers"

	"k8s.io/klog/v2"
)

const (
	// GNPFinalizer - finalizer value placed on GNP objects by GNP Controller
	GNPFinalizer  = "networking.gke.io/gnp-controller"
	gnpKind       = "gkenetworkparamset"
	workqueueName = "gkenetworkparamset"
)

// Controller manages GKENetworkParamSet status.
type Controller struct {
	gkeNetworkParamsInformer networkinformer.GKENetworkParamSetInformer
	networkInformer          networkinformer.NetworkInformer
	networkClientset         networkclientset.Interface
	gceCloud                 *gce.Cloud
	queue                    workqueue.RateLimitingInterface
	networkInformerFactory   networkinformers.SharedInformerFactory
}

// NewGKENetworkParamSetController returns a new
func NewGKENetworkParamSetController(
	networkClientset networkclientset.Interface,
	gkeNetworkParamsInformer networkinformer.GKENetworkParamSetInformer,
	networkInformer networkinformer.NetworkInformer,
	gceCloud *gce.Cloud,
	networkInformerFactory networkinformers.SharedInformerFactory,
) *Controller {

	// register GNP metrics
	registerGKENetworkParamSetMetrics()

	return &Controller{
		networkClientset:         networkClientset,
		gkeNetworkParamsInformer: gkeNetworkParamsInformer,
		networkInformer:          networkInformer,
		gceCloud:                 gceCloud,
		queue:                    workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: workqueueName}),
		networkInformerFactory:   networkInformerFactory,
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

	gnpInf := c.gkeNetworkParamsInformer.Informer()
	gnpInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
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
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
	})

	// network.Spec.ParametersRef has 3 cases.
	// nil (when the network resource is backed without a managed cloud environment like gcp)
	// not nil, but points to a different type of params object (could eventually be something like awsParams)
	// not nil and points to a GNP object (We want to process to these)

	nwInf := c.networkInformer.Informer()
	nwInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			network := obj.(*networkv1.Network)
			if network.Spec.ParametersRef != nil && strings.EqualFold(network.Spec.ParametersRef.Kind, gnpKind) {
				c.queue.Add(network.Spec.ParametersRef.Name)
			}
		},
		// this could result in a large amount of updates, but we cap the number of possible networks to avoid those issues
		UpdateFunc: func(old, new interface{}) {
			newNetwork := new.(*networkv1.Network)
			if newNetwork.Spec.ParametersRef != nil && strings.EqualFold(newNetwork.Spec.ParametersRef.Kind, gnpKind) {
				c.queue.Add(newNetwork.Spec.ParametersRef.Name)
			}

			// we need to check the old network to see if we are no longer referencing the same GNP
			// this is important so we can delete a GNP waiting for a Network to no longer be inuse.
			oldNetwork := old.(*networkv1.Network)
			if oldNetwork.Spec.ParametersRef != nil && strings.EqualFold(oldNetwork.Spec.ParametersRef.Kind, gnpKind) {
				if newNetwork.Spec.ParametersRef == nil || !strings.EqualFold(newNetwork.Spec.ParametersRef.Kind, gnpKind) || oldNetwork.Spec.ParametersRef.Name != newNetwork.Spec.ParametersRef.Name {
					c.queue.Add(oldNetwork.Spec.ParametersRef.Name)
				}
			}
		},

		DeleteFunc: func(obj interface{}) {
			network := obj.(*networkv1.Network)
			if network.Spec.ParametersRef != nil && strings.EqualFold(network.Spec.ParametersRef.Kind, gnpKind) {
				c.queue.Add(network.Spec.ParametersRef.Name)
			}
		},
	})

	c.networkInformerFactory.Start(stopCh)

	if !cache.WaitForNamedCacheSync("gkenetworkparamset", stopCh, nwInf.HasSynced, gnpInf.HasSynced) {
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

	err := c.reconcile(ctx, key.(string))
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
	controllermetrics.WorkqueueDroppedObjects.WithLabelValues(workqueueName).Inc()
}

// addFinalizerInPlace adds a finalizer by mutating params if it doesnt already exist
func addFinalizerInPlace(params *networkv1.GKENetworkParamSet) {
	gnpFinalizers := params.GetFinalizers()
	for _, f := range gnpFinalizers {
		if f == GNPFinalizer {
			return
		}
	}

	params.SetFinalizers(append(gnpFinalizers, GNPFinalizer))
}

// removeFinalizerInPlace removes a finalizer by mutating params if the finalizer exists
func removeFinalizerInPlace(params *networkv1.GKENetworkParamSet) {
	finalizers := params.GetFinalizers()
	for i, f := range finalizers {
		if f == GNPFinalizer {
			finalizers = append(finalizers[:i], finalizers[i+1:]...)
			break
		}
	}

	params.SetFinalizers(finalizers)
}

func (c *Controller) reconcile(ctx context.Context, key string) error {
	originalParams, err := c.gkeNetworkParamsInformer.Lister().Get(key)

	if err != nil {
		if errors.IsNotFound(err) {
			return c.cleanupGNPDeletion(ctx, key) // GNP was deleted, run cleanup
		}
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	params := originalParams.DeepCopy()

	err = c.syncGNP(ctx, params)

	if !reflect.DeepEqual(originalParams.Status, params.Status) {
		if updateErr := c.updateGKENetworkParamSetStatus(ctx, params); updateErr != nil {
			err = multierror.Append(updateErr, err)
		}
		if updateErr := c.updateGKENetworkParamSet(ctx, params); updateErr != nil {
			err = multierror.Append(updateErr, err)
		}
	} else if !reflect.DeepEqual(originalParams, params) {
		if updateErr := c.updateGKENetworkParamSet(ctx, params); updateErr != nil {
			err = multierror.Append(updateErr, err)
		}
	}

	if err != nil {
		return err
	}

	gnpObjects.WithLabelValues(strconv.FormatBool(meta.IsStatusConditionTrue(originalParams.Status.Conditions, string(networkv1.GKENetworkParamSetStatusReady))), string(originalParams.Spec.DeviceMode)).Dec()
	gnpObjects.WithLabelValues(strconv.FormatBool(meta.IsStatusConditionTrue(params.Status.Conditions, string(networkv1.GKENetworkParamSetStatusReady))), string(params.Spec.DeviceMode)).Inc()

	return nil
}

// syncGNP transforms GNP, but does not update it in cluster.
// Manages corresponding network update if there is a Network referencing this GNP
func (c *Controller) syncGNP(ctx context.Context, params *networkv1.GKENetworkParamSet) error {
	if params.DeletionTimestamp != nil {
		// GKENetworkParamSet is being deleted, handle the delete event
		return c.handleGNPDelete(ctx, params)
	}

	addFinalizerInPlace(params)

	subnet, subnetValidation := c.getAndValidateSubnet(ctx, params)
	meta.SetStatusCondition(&params.Status.Conditions, subnetValidation.toCondition())
	if !subnetValidation.IsValid {
		return nil
	}

	paramsValidation, err := c.validateGKENetworkParamSet(ctx, params, subnet)
	if err != nil {
		return err
	}
	meta.SetStatusCondition(&params.Status.Conditions, paramsValidation.toCondition())
	if !paramsValidation.IsValid {
		return nil
	}

	cidrs := extractRelevantCidrs(subnet, params)
	params.Status.PodCIDRs = &networkv1.NetworkRanges{
		CIDRBlocks: cidrs,
	}

	network, err := c.getNetworkReferringToGNP(params.Name)
	if err != nil {
		return err
	}
	if network == nil {
		return nil
	}

	err = c.syncNetworkWithGNP(ctx, network, params)
	if err != nil {
		return err
	}
	return nil
}

// getNetworkReferringToGNP returns the Network that references the GNP name, or nil if none exist
func (c *Controller) getNetworkReferringToGNP(gnpName string) (*networkv1.Network, error) {
	networks, err := c.networkInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	// see if one of the networks is referencing this GNP
	for _, network := range networks {
		if network.Spec.ParametersRef != nil && network.Spec.ParametersRef.Name == gnpName && strings.EqualFold(network.Spec.ParametersRef.Kind, gnpKind) {
			return network, nil
		}
	}
	return nil, nil
}

// syncNetworkWithGNP does the cross sync of Network with GNP.
// GNP can be mutated, while a copy of Network is both transformed AND updated in the cluster
func (c *Controller) syncNetworkWithGNP(ctx context.Context, network *networkv1.Network, params *networkv1.GKENetworkParamSet) error {
	newNetwork := network.DeepCopy()

	networkCrossValidation := crossValidateNetworkAndGnp(newNetwork, params)
	meta.SetStatusCondition(&newNetwork.Status.Conditions, networkCrossValidation.toCondition())
	if !reflect.DeepEqual(newNetwork.Status.Conditions, network.Status.Conditions) {
		_, err := c.networkClientset.NetworkingV1().Networks().UpdateStatus(ctx, newNetwork, v1.UpdateOptions{})
		if err != nil {
			return err
		}

	}

	if !networkCrossValidation.IsValid {
		return nil
	}

	params.Status.NetworkName = newNetwork.Name
	return nil
}

// handleGNPDelete checks to see if its safe to delete the GNP resource before calling executeGNPDelete on it
func (c *Controller) handleGNPDelete(ctx context.Context, params *networkv1.GKENetworkParamSet) error {
	if params.Status.NetworkName == "" {
		return c.executeGNPDelete(ctx, params, nil)
	}

	network, err := c.networkClientset.NetworkingV1().Networks().Get(ctx, params.Status.NetworkName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.executeGNPDelete(ctx, params, nil)
		}
		return err
	}

	networkStillRefersToGNP := network.Spec.ParametersRef != nil && strings.EqualFold(network.Spec.ParametersRef.Kind, gnpKind) && network.Spec.ParametersRef.Name == params.Name
	if !networkStillRefersToGNP {
		return c.executeGNPDelete(ctx, params, network)
	}

	if networkStillRefersToGNP && !network.InUse() {
		return c.executeGNPDelete(ctx, params, network)
	}

	// if the network is in use, this GNP object will get reconciled again when the network's in use status changes.

	return nil
}

func (c *Controller) executeGNPDelete(ctx context.Context, params *networkv1.GKENetworkParamSet, network *networkv1.Network) error {
	removeFinalizerInPlace(params)

	return nil
}

// cleanupGNPDeletion is called post GNP deletion
func (c *Controller) cleanupGNPDeletion(ctx context.Context, gnpName string) error {
	network, err := c.getNetworkReferringToGNP(gnpName)
	if err != nil {
		return err
	}
	if network == nil {
		return nil
	}

	newNetwork := network.DeepCopy()
	meta.SetStatusCondition(&newNetwork.Status.Conditions, v1.Condition{
		Type:    string(networkv1.NetworkConditionStatusParamsReady),
		Status:  v1.ConditionFalse,
		Reason:  string(networkv1.GNPDeleted),
		Message: fmt.Sprintf("GKENetworkParamSet resource was deleted: %v", gnpName),
	})
	if _, err := c.networkClientset.NetworkingV1().Networks().UpdateStatus(ctx, newNetwork, v1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// extractRelevantCidrs returns the CIDRS of the named ranges in paramset
func extractRelevantCidrs(subnet *compute.Subnetwork, paramset *networkv1.GKENetworkParamSet) []string {
	cidrs := []string{}

	// use the subnet cidr if there are no secondary ranges specified by user in params, this can only happen if the GNP is using deviceMode
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

func paramSetIncludesRange(params *networkv1.GKENetworkParamSet, secondaryRangeName string) bool {
	for _, rn := range params.Spec.PodIPv4Ranges.RangeNames {
		if rn == secondaryRangeName {
			return true
		}
	}
	return false
}

func (c *Controller) updateGKENetworkParamSet(ctx context.Context, params *networkv1.GKENetworkParamSet) error {
	_, err := c.networkClientset.NetworkingV1().GKENetworkParamSets().Update(ctx, params, v1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update GKENetworkParamSet: %w", err)
	}
	return nil
}

func (c *Controller) updateGKENetworkParamSetStatus(ctx context.Context, params *networkv1.GKENetworkParamSet) error {
	_, err := c.networkClientset.NetworkingV1().GKENetworkParamSets().UpdateStatus(ctx, params, v1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update GKENetworkParamSet Status: %w", err)
	}
	return nil
}
