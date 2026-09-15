//go:build !providerless
// +build !providerless

/*
Copyright 2017 The Kubernetes Authors.

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
	"flag"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/klog/v2"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	cloudprovider "k8s.io/cloud-provider"
	netutils "k8s.io/utils/net"
)

type cidrs struct {
	ipn   netutils.IPNetSet
	isSet bool
}

type lbSyncResult struct {
	status      *v1.LoadBalancerStatus
	annotations map[string]string
}

func newLBSyncResult() *lbSyncResult {
	annotations := make(map[string]string, len(l4ResourceAnnotationKeys))

	for _, key := range l4ResourceAnnotationKeys {
		annotations[key] = "" // Initialize to empty string to indicate deletion by default if not set later
	}

	return &lbSyncResult{
		annotations: annotations,
	}
}

var (
	l4LbSrcRngsFlag cidrs
	l7lbSrcRngsFlag cidrs

	overrideL4ILBHealthCheckSourceCIDRs   string
	overrideL4NetLBHealthCheckSourceCIDRs string
)

// SetOverrideL4ILBHealthCheckSourceCIDRs sets the override source CIDRs for L4 ILB health checks.
func SetOverrideL4ILBHealthCheckSourceCIDRs(cidrs string) {
	overrideL4ILBHealthCheckSourceCIDRs = cidrs
}

// SetOverrideL4NetLBHealthCheckSourceCIDRs sets the override source CIDRs for L4 NetLB health checks.
func SetOverrideL4NetLBHealthCheckSourceCIDRs(cidrs string) {
	overrideL4NetLBHealthCheckSourceCIDRs = cidrs
}

func init() {
	var err error
	// L3/4 health checkers have client addresses within these known CIDRs.
	l4LbSrcRngsFlag.ipn, err = netutils.ParseIPNets([]string{"130.211.0.0/22", "35.191.0.0/16", "209.85.152.0/22", "209.85.204.0/22"}...)
	if err != nil {
		panic("Incorrect default GCE L3/4 source ranges")
	}
	// L7 health checkers have client addresses within these known CIDRs.
	l7lbSrcRngsFlag.ipn, err = netutils.ParseIPNets([]string{"130.211.0.0/22", "35.191.0.0/16"}...)
	if err != nil {
		panic("Incorrect default GCE L7 source ranges")
	}

	flag.Var(&l4LbSrcRngsFlag, "cloud-provider-gce-lb-src-cidrs", "CIDRs opened in GCE firewall for L4 LB traffic proxy & health checks")
	flag.Var(&l7lbSrcRngsFlag, "cloud-provider-gce-l7lb-src-cidrs", "CIDRs opened in GCE firewall for L7 LB traffic proxy & health checks")
}

// String is the method to format the flag's value, part of the flag.Value interface.
func (c *cidrs) String() string {
	s := c.ipn.StringSlice()
	sort.Strings(s)
	return strings.Join(s, ",")
}

// Set supports a value of CSV or the flag repeated multiple times
func (c *cidrs) Set(value string) error {
	// On first Set(), clear the original defaults
	if !c.isSet {
		c.isSet = true
		c.ipn = make(netutils.IPNetSet)
	} else {
		return fmt.Errorf("GCE LB CIDRs have already been set")
	}

	for _, cidr := range strings.Split(value, ",") {
		_, ipnet, err := netutils.ParseCIDRSloppy(cidr)
		if err != nil {
			return err
		}

		c.ipn.Insert(ipnet)
	}
	return nil
}

// parseHealthCheckCIDRs parses a comma-separated string of CIDRs and returns an IPNetSet.
// Invalid CIDRs are logged and skipped.
func parseHealthCheckCIDRs(cidrs string) netutils.IPNetSet {
	if cidrs == "" {
		return nil
	}
	ipNetSet := make(netutils.IPNetSet)
	for _, c := range strings.Split(cidrs, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			klog.Warningf("Ignoring invalid health check CIDR: %v", c)
			continue
		}
		ipNetSet.Insert(ipnet)
	}
	return ipNetSet
}

func filterIPNetSetByFamily(ipns netutils.IPNetSet, isIPv6 bool) netutils.IPNetSet {
	filtered := make(netutils.IPNetSet)
	for _, ipn := range ipns {
		if netutils.IsIPv6(ipn.IP) == isIPv6 {
			filtered.Insert(ipn)
		}
	}
	return filtered
}

// L4LBType represents the type of L4 load balancer.
type L4LBType string

const (
	// L4LBTypeILB is the constant for Internal Load Balancer.
	L4LBTypeILB L4LBType = "ILB"
	// L4LBTypeNetLB is the constant for Network Load Balancer.
	L4LBTypeNetLB L4LBType = "NetLB"
)

// getHCFirewallSourceRanges returns the list of source CIDR ranges for the health check firewall rule.
//
// Arguments:
// l4Type: The type of L4 load balancer (ILB or XLB/NetLB).
// shared: Indicates whether the firewall rule is shared between different K8s Services (e.g., when ExternalTrafficPolicy=Cluster).
//
//	If shared is true, the firewall rule must include the ranges for both ILB and NetLB because a single global firewall rule is used.
//
// isIPv6: Specifies if we are retrieving ranges for an IPv6 firewall rule. If false, we return IPv4 ranges.
//
// Logic:
// The function gathers the appropriate CIDR ranges based on the LB type(s). If 'shared' is true, it aggregates
// the CIDRs for both ILB and NetLB.
// For each LB type, it parses the override flag (if provided) and extracts the requested IP family.
// If the flag is not provided, or if the extracted ranges are empty (meaning the flag was missing that specific IP family),
// it falls back to the default GCP health check ranges (l4LbSrcRngsFlag). Note that these default ranges only
// contain IPv4 addresses because the controller's default configuration does not currently support IPv6.
// Finally, it removes duplicates from the aggregated ranges.
func getHCFirewallSourceRanges(l4Type L4LBType, shared bool, isIPv6 bool) netutils.IPNetSet {
	ranges := make(netutils.IPNetSet)

	lbTypesToProcess := []L4LBType{l4Type}
	if shared {
		lbTypesToProcess = []L4LBType{L4LBTypeILB, L4LBTypeNetLB}
	}

	for _, lbType := range lbTypesToProcess {
		currentRanges := make(netutils.IPNetSet)

		// use custom HC ranges if provided (could contain IPv4/v6 ranges)
		var overrideStr string
		switch lbType {
		case L4LBTypeILB:
			overrideStr = overrideL4ILBHealthCheckSourceCIDRs
		case L4LBTypeNetLB:
			overrideStr = overrideL4NetLBHealthCheckSourceCIDRs
		}
		if overrideStr != "" {
			ipns := parseHealthCheckCIDRs(overrideStr)
			currentRanges = filterIPNetSetByFamily(ipns, isIPv6)
		}

		// use the default HC ranges
		if len(currentRanges) == 0 {
			// l4LbSrcRngsFlag contains IPv4 only ranges for both ILB and NetLB
			// Controller does not support IPv6
			currentRanges = filterIPNetSetByFamily(l4LbSrcRngsFlag.ipn, isIPv6)
		}

		for _, ipn := range currentRanges {
			ranges.Insert(ipn)
		}
	}

	return ranges
}

// L4ILBHealthCheckSrcRanges returns the ranges of ips used by the GCE L4 ILB load balancers
// for performing health checks.
func L4ILBHealthCheckSrcRanges(shared bool, isIPv6 bool) netutils.IPNetSet {
	return getHCFirewallSourceRanges(L4LBTypeILB, shared, isIPv6)
}

// L4NetLBHealthCheckSrcRanges returns the ranges of ips used by the GCE L4 NetLB load balancers
// for performing health checks.
func L4NetLBHealthCheckSrcRanges(shared bool, isIPv6 bool) netutils.IPNetSet {
	return getHCFirewallSourceRanges(L4LBTypeNetLB, shared, isIPv6)
}

// L4LoadBalancerSrcRanges contains the ranges of ips used by the L3/L4 GCE load balancers
// for proxying client requests and performing health checks.
func L4LoadBalancerSrcRanges() []string {
	return l4LbSrcRngsFlag.ipn.StringSlice()
}

// L7LoadBalancerSrcRanges contains the ranges of ips used by the GCE load balancers L7
// for proxying client requests and performing health checks.
func L7LoadBalancerSrcRanges() []string {
	return l7lbSrcRngsFlag.ipn.StringSlice()
}

// GetLoadBalancer is an implementation of LoadBalancer.GetLoadBalancer
func (g *Cloud) GetLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	loadBalancerName := g.GetLoadBalancerName(ctx, clusterName, svc)
	fwd, err := g.GetRegionForwardingRule(loadBalancerName, g.region)
	if err == nil {
		status := &v1.LoadBalancerStatus{}
		status.Ingress = []v1.LoadBalancerIngress{{IP: fwd.IPAddress}}

		return status, true, nil
	}
	// Checking for finalizer is more accurate because controller restart could happen in the middle of resource
	// deletion. So even though forwarding rule was deleted, cleanup might not have been complete.
	if hasFinalizer(svc, ILBFinalizerV1) || hasFinalizer(svc, NetLBFinalizerV1) {
		return &v1.LoadBalancerStatus{}, true, nil
	}
	return nil, false, ignoreNotFound(err)
}

// GetLoadBalancerName is an implementation of LoadBalancer.GetLoadBalancerName.
func (g *Cloud) GetLoadBalancerName(ctx context.Context, clusterName string, svc *v1.Service) string {
	// TODO: replace DefaultLoadBalancerName to generate more meaningful loadbalancer names.
	return cloudprovider.DefaultLoadBalancerName(svc)
}

// EnsureLoadBalancer is an implementation of LoadBalancer.EnsureLoadBalancer.
func (g *Cloud) EnsureLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	// Ignore services with LoadBalancerClass different than "networking.gke.io/l4-regional-external-legacy" or
	// "networking.gke.io/l4-regional-internal-legacy" used for these controllers.
	// LoadBalancerClass can't be updated (see the field API doc) so we don't need to clean any resources.
	if svc.Spec.LoadBalancerClass != nil && !hasLoadBalancerClass(svc, LegacyRegionalInternalLoadBalancerClass) && !hasLoadBalancerClass(svc, LegacyRegionalExternalLoadBalancerClass) {
		klog.Infof("Ignoring service %s/%s using load balancer class %q, it is not supported by this controller.", svc.Namespace, svc.Name, *svc.Spec.LoadBalancerClass)
		return nil, cloudprovider.ImplementedElsewhere
	}

	loadBalancerName := g.GetLoadBalancerName(ctx, clusterName, svc)
	desiredScheme := getSvcScheme(svc)
	clusterID, err := g.ClusterID.GetID()
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("EnsureLoadBalancer(%v, %v, %v, %v, %v): ensure %v loadbalancer", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, desiredScheme)

	existingFwdRule, err := g.GetRegionForwardingRule(loadBalancerName, g.region)
	if err != nil && !isNotFound(err) {
		return nil, err
	}

	if existingFwdRule != nil {
		existingScheme := cloud.LbScheme(strings.ToUpper(existingFwdRule.LoadBalancingScheme))

		// If the loadbalancer type changes between INTERNAL and EXTERNAL, the old load balancer should be deleted.
		if existingScheme != desiredScheme {
			klog.V(4).Infof("EnsureLoadBalancer(%v, %v, %v, %v, %v): deleting existing %v loadbalancer", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, existingScheme)
			switch existingScheme {
			case cloud.SchemeInternal:
				err = g.ensureInternalLoadBalancerDeleted(clusterName, clusterID, svc)
			default:
				err = g.ensureExternalLoadBalancerDeleted(clusterName, clusterID, svc)
			}
			klog.V(4).Infof("EnsureLoadBalancer(%v, %v, %v, %v, %v): done deleting existing %v loadbalancer. err: %v", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, existingScheme, err)
			if err != nil {
				return nil, err
			}

			// Assume the ensureDeleted function successfully deleted the forwarding rule.
			existingFwdRule = nil
		}
	}

	var syncResult *lbSyncResult
	switch desiredScheme {
	case cloud.SchemeInternal:
		syncResult, err = g.ensureInternalLoadBalancer(clusterName, clusterID, svc, existingFwdRule, nodes)
	default:
		syncResult, err = g.ensureExternalLoadBalancer(clusterName, clusterID, svc, existingFwdRule, nodes)
	}
	if err != nil {
		klog.Errorf("Failed to EnsureLoadBalancer(%s, %s, %s, %s, %s), err: %v", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, err)
		return nil, err
	}

	var status *v1.LoadBalancerStatus
	var annotations map[string]string

	if syncResult != nil {
		status = syncResult.status
		annotations = syncResult.annotations
	}

	if g.enableL4LBAnnotations {
		if err = g.updateL4ResourcesAnnotations(ctx, svc, annotations); err != nil {
			return status, fmt.Errorf("failed to set resource annotations, err: %w", err)
		}
	}

	klog.V(4).Infof("EnsureLoadBalancer(%s, %s, %s, %s, %s): done ensuring loadbalancer.", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region)
	return status, err
}

func (g *Cloud) updateL4ResourcesAnnotations(ctx context.Context, svc *v1.Service, newL4LBAnnotations map[string]string) error {
	newObjectMetadata, shouldUpdate := computeNewAnnotationsIfNeeded(svc, newL4LBAnnotations)
	if !shouldUpdate {
		return nil
	}
	newSvc := svc.DeepCopy()
	newSvc.ObjectMeta = *newObjectMetadata

	patchBytes, err := servicePatchBytes(svc, newSvc)
	if err != nil {
		return err
	}

	_, err = g.client.CoreV1().Services(svc.Namespace).Patch(ctx, svc.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")

	return err
}

func servicePatchBytes(oldSvc, newSvc *v1.Service) ([]byte, error) {
	oldData, err := json.Marshal(oldSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal oldData for svc %s/%s: %v", oldSvc.Namespace, oldSvc.Name, err)
	}

	newData, err := json.Marshal(newSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal newData for svc %s/%s: %v", newSvc.Namespace, newSvc.Name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Service{})
	if err != nil {
		return nil, fmt.Errorf("failed to CreateTwoWayMergePatch for svc %s/%s: %v", oldSvc.Namespace, oldSvc.Name, err)
	}
	return patchBytes, nil
}

// UpdateLoadBalancer is an implementation of LoadBalancer.UpdateLoadBalancer.
func (g *Cloud) UpdateLoadBalancer(ctx context.Context, clusterName string, svc *v1.Service, nodes []*v1.Node) error {
	// Ignore services with LoadBalancerClass different than "networking.gke.io/l4-regional-external-legacy" or
	// "networking.gke.io/l4-regional-internal-legacy" used for these controllers.
	// LoadBalancerClass can't be updated (see the field API doc) so we don't need to clean any resources.
	if svc.Spec.LoadBalancerClass != nil && !hasLoadBalancerClass(svc, LegacyRegionalInternalLoadBalancerClass) && !hasLoadBalancerClass(svc, LegacyRegionalExternalLoadBalancerClass) {
		klog.Infof("Ignoring service %s/%s using load balancer class %q, it is not supported by this controller.", svc.Namespace, svc.Name, *svc.Spec.LoadBalancerClass)
		return cloudprovider.ImplementedElsewhere
	}

	loadBalancerName := g.GetLoadBalancerName(ctx, clusterName, svc)
	scheme := getSvcScheme(svc)
	clusterID, err := g.ClusterID.GetID()
	if err != nil {
		return err
	}

	klog.V(4).Infof("UpdateLoadBalancer(%v, %v, %v, %v, %v): updating with %v nodes [node names limited, total number of nodes: %d]", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, loggableNodeNames(nodes), len(nodes))

	switch scheme {
	case cloud.SchemeInternal:
		err = g.updateInternalLoadBalancer(clusterName, clusterID, svc, nodes)
	default:
		err = g.updateExternalLoadBalancer(clusterName, svc, nodes)
	}
	klog.V(4).Infof("UpdateLoadBalancer(%v, %v, %v, %v, %v): done updating. err: %v", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, err)
	return err
}

// EnsureLoadBalancerDeleted is an implementation of LoadBalancer.EnsureLoadBalancerDeleted.
func (g *Cloud) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, svc *v1.Service) error {
	// Ignore services with LoadBalancerClass different than "networking.gke.io/l4-regional-external-legacy" or
	// "networking.gke.io/l4-regional-internal-legacy" used for these controllers.
	// LoadBalancerClass can't be updated (see the field API doc) so we don't need to clean any resources.
	if svc.Spec.LoadBalancerClass != nil && !hasLoadBalancerClass(svc, LegacyRegionalInternalLoadBalancerClass) && !hasLoadBalancerClass(svc, LegacyRegionalExternalLoadBalancerClass) {
		klog.Infof("Ignoring service %s/%s using load balancer class %q, it is not supported by this controller.", svc.Namespace, svc.Name, *svc.Spec.LoadBalancerClass)
		return cloudprovider.ImplementedElsewhere
	}

	loadBalancerName := g.GetLoadBalancerName(ctx, clusterName, svc)
	scheme := getSvcScheme(svc)
	clusterID, err := g.ClusterID.GetID()
	if err != nil {
		return err
	}

	klog.V(4).Infof("EnsureLoadBalancerDeleted(%v, %v, %v, %v, %v): deleting loadbalancer", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region)

	switch scheme {
	case cloud.SchemeInternal:
		err = g.ensureInternalLoadBalancerDeleted(clusterName, clusterID, svc)
	default:
		err = g.ensureExternalLoadBalancerDeleted(clusterName, clusterID, svc)
	}
	klog.V(4).Infof("EnsureLoadBalancerDeleted(%v, %v, %v, %v, %v): done deleting loadbalancer. err: %v", clusterName, svc.Namespace, svc.Name, loadBalancerName, g.region, err)
	return err
}

func getSvcScheme(svc *v1.Service) cloud.LbScheme {
	if t := GetLoadBalancerAnnotationType(svc); t == LBTypeInternal {
		return cloud.SchemeInternal
	}
	return cloud.SchemeExternal
}

// checkMixedProtocol checks if the Service Ports uses different protocols,
// per examples, TCP and UDP.
func checkMixedProtocol(ports []v1.ServicePort) error {
	if len(ports) == 0 {
		return nil
	}

	firstProtocol := ports[0].Protocol
	for _, port := range ports[1:] {
		if port.Protocol != firstProtocol {
			return fmt.Errorf("mixed protocol is not supported for LoadBalancer")
		}
	}
	return nil
}

// hasLoadBalancerPortsError checks if the Service has the LoadBalancerPortsError set to True
func hasLoadBalancerPortsError(service *v1.Service) bool {
	if service == nil {
		return false
	}

	for _, cond := range service.Status.Conditions {
		if cond.Type == v1.LoadBalancerPortsError {
			return cond.Status == metav1.ConditionTrue
		}
	}
	return false
}

// computeNewAnnotationsIfNeeded checks if new annotations should be added to service.
// If needed creates new service meta object.
// This function is used by L4 LB controllers.
func computeNewAnnotationsIfNeeded(svc *v1.Service, newAnnotations map[string]string) (*metav1.ObjectMeta, bool) {
	newObjectMeta := svc.ObjectMeta.DeepCopy()
	newObjectMeta.Annotations = mergeMap(newObjectMeta.Annotations, newAnnotations)
	if reflect.DeepEqual(svc.Annotations, newObjectMeta.Annotations) {
		return nil, false
	}
	return newObjectMeta, true
}

// processMixedProtocolCheck checks if the Service Ports use different protocols and updates
// the corresponding Service Status Condition.
//
// Services with multiples protocols are not supported by this controller, warn the users and sets
// the corresponding Service Status Condition.
// https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/1435-mixed-protocol-lb
//
// For updates we want to keep processing to not break them.
//
// Originally introduced in https://github.com/kubernetes/cloud-provider-gcp/pull/475
func (g *Cloud) processMixedProtocolCheck(ctx context.Context, svc *v1.Service, isUpdate bool) error {
	err := checkMixedProtocol(svc.Spec.Ports)
	if err == nil {
		return nil
	}
	if hasLoadBalancerPortsError(svc) {
		if isUpdate {
			return nil
		}
		return err
	}

	klog.Warningf("Ignoring %s/%s using different ports protocols, isUpdate: %t", svc.Namespace, svc.Name, isUpdate)

	if g.eventRecorder != nil {
		g.eventRecorder.Event(svc, v1.EventTypeWarning, v1.LoadBalancerPortsErrorReason, "LoadBalancer with multiple protocols are not supported.")
	}

	svcApplyStatus := corev1apply.ServiceStatus().WithConditions(
		metav1apply.Condition().
			WithType(v1.LoadBalancerPortsError).
			WithStatus(metav1.ConditionTrue).
			WithReason(v1.LoadBalancerPortsErrorReason).
			WithLastTransitionTime(metav1.Now()).
			WithMessage("LoadBalancer with multiple protocols are not supported"))
	svcApply := corev1apply.Service(svc.Name, svc.Namespace).WithStatus(svcApplyStatus)

	if _, errApply := g.client.CoreV1().Services(svc.Namespace).ApplyStatus(ctx, svcApply, metav1.ApplyOptions{FieldManager: "gce-cloud-controller", Force: true}); errApply != nil {
		return errApply
	}

	if isUpdate {
		return nil
	}
	return err
}
