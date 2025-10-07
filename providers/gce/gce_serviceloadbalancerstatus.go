package gce

import (
	"context"
	"reflect"

	svclbstatus "github.com/GoogleCloudPlatform/gke-networking-api/apis/serviceloadbalancerstatus/v1"
	svclbstatusclient "github.com/GoogleCloudPlatform/gke-networking-api/client/serviceloadbalancerstatus/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// This file contains functions for managing the ServiceLoadBalancerStatus CRD

func (g *Cloud) InitializeServiceLoadBalancerStatusCRD(kubeConfig *restclient.Config) error {
	kubeConfig.ContentType = "application/json"
	SvcLBStatusClient, err := svclbstatusclient.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	g.serviceLBStatusClient = SvcLBStatusClient
	return nil
}

// serviceLoadBalancerStatusSpecEqual checks if two ServiceLoadBalancerStatusSpec are equal.
// It compares GceResources as a multiset (bag), ignoring order.
func (g *Cloud) serviceLoadBalancerStatusSpecEqual(a, b svclbstatus.ServiceLoadBalancerStatusSpec) bool {
	if !reflect.DeepEqual(a.ServiceRef, b.ServiceRef) {
		return false
	}

	if len(a.GceResources) != len(b.GceResources) {
		return false
	}

	resourceCounts := make(map[string]int, len(a.GceResources))
	for _, res := range a.GceResources {
		resourceCounts[res]++
	}

	for _, res := range b.GceResources {
		if count, ok := resourceCounts[res]; !ok || count == 0 {
			return false
		}
		resourceCounts[res]--
	}

	return true
}

// GenerateServiceLoadBalancerStatus creates a ServiceLoadBalancerStatus CR from a Service and GCEResources.
func (g *Cloud) generateServiceLoadBalancerStatus(service *v1.Service, gceResources []string) *svclbstatus.ServiceLoadBalancerStatus {
	if service == nil {
		return nil
	}

	statusCR := &svclbstatus.ServiceLoadBalancerStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name + "-status",
			Namespace: service.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(service, v1.SchemeGroupVersion.WithKind("Service")),
			},
		},
		Spec: svclbstatus.ServiceLoadBalancerStatusSpec{
			ServiceRef: svclbstatus.ServiceReference{
				Kind:       "Service",
				Name:       service.Name,
				APIVersion: v1.SchemeGroupVersion.String(),
			},
			GceResources: gceResources,
		},
		Status: svclbstatus.ServiceLoadBalancerStatusStatus{},
	}

	return statusCR
}

// ensureServiceLoadBalancerStatusCR ensures the ServiceLoadBalancerStatus CR
// exists for the given Service and that its Spec is up-to-date.
// If the list of GCE resources is empty, it ensures the CR is deleted.
func (g *Cloud) EnsureServiceLoadBalancerStatusCR(service *v1.Service, gceResourceURLs []string) error {
	// Generate the desired state of the CR.
	desiredCR := g.generateServiceLoadBalancerStatus(service, gceResourceURLs)
	if desiredCR == nil {
		klog.V(4).Info("Generated ServiceLoadBalancerStatus CR is nil, skipping")
		return nil
	}
	crClient := g.serviceLBStatusClient.NetworkingV1().ServiceLoadBalancerStatuses(service.Namespace)

	// Try to get the existing CR from the cluster.
	existingCR, err := crClient.Get(context.TODO(), desiredCR.Name, metav1.GetOptions{})
	if err != nil {
		// If the CR does not exist, create it.
		if errors.IsNotFound(err) {
			klog.V(2).Info("ServiceLoadBalancerStatus CR not found, creating it", "crName", desiredCR.Name, "spec", desiredCR.Spec)
			_, createErr := crClient.Create(context.TODO(), desiredCR, metav1.CreateOptions{})
			if createErr != nil {
				klog.Error(createErr, "Failed to create ServiceLoadBalancerStatus CR", "crName", desiredCR.Name)
				return createErr
			}
			klog.V(2).Info("Successfully created ServiceLoadBalancerStatus CR", "crName", desiredCR.Name)
			return nil
		}
		// For any other error, log and return it.
		klog.Error(err, "Failed to get ServiceLoadBalancerStatus CR", "crName", desiredCR.Name)
		return err
	}

	// If the CR already exists, check if an update is needed.
	if g.serviceLoadBalancerStatusSpecEqual(existingCR.Spec, desiredCR.Spec) {
		klog.V(3).Info("ServiceLoadBalancerStatus CR is already up-to-date", "crName", desiredCR.Name)
		return nil
	}

	// The Spec has changed, so update the CR.
	klog.V(2).Info("ServiceLoadBalancerStatus CR spec has changed, updating it", "crName", desiredCR.Name)
	klog.V(2).Info("Updating ServiceLoadBalancerStatus CR with new spec", "newSpec", desiredCR.Spec)
	// To update, we modify the existing object's Spec and send it back.
	existingCR.Spec = desiredCR.Spec
	_, updateErr := crClient.Update(context.TODO(), existingCR, metav1.UpdateOptions{})
	if updateErr != nil {
		klog.Error(updateErr, "Failed to update ServiceLoadBalancerStatus CR", "crName", desiredCR.Name)
		return updateErr
	}

	klog.V(2).Info("Successfully updated ServiceLoadBalancerStatus CR", "crName", desiredCR.Name)
	return nil
}
