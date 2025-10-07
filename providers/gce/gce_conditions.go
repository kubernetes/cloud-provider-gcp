package gce

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	L4LBBackendServiceConditionType = "ServiceLoadBalancerBackendService"

	L4LBTargetPoolConditionType = "ServiceLoadBalancerTargetPool"

	L4LBForwardingRuleConditionType          = "ServiceLoadBalancerForwardingRule"
	L4LBHealthCheckConditionType             = "ServiceLoadBalancerHealthCheck"
	L4LBFirewallRuleConditionType            = "ServiceLoadBalancerFirewallRule"
	L4LBFirewallRuleHealthCheckConditionType = "ServiceLoadBalancerFirewallRuleForHealthCheck"

	L4LBConditionReason = "GCEResourceAllocated"
)

func NewConditionResourceAllocated(conditionType string, resourceName string) metav1.Condition {
	return metav1.Condition{
		LastTransitionTime: metav1.Now(),
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             L4LBConditionReason,
		Message:            resourceName,
	}
}

// NewBackendServiceCondition creates a condition for the backend service.
func NewBackendServiceCondition(bsName string) metav1.Condition {
	return NewConditionResourceAllocated(L4LBBackendServiceConditionType, bsName)
}

// NewTargetPoolCondition creates a condition for the backend service.
func NewTargetPoolCondition(tpName string) metav1.Condition {
	return NewConditionResourceAllocated(L4LBTargetPoolConditionType, tpName)
}

// NewForwardingRuleCondition creates a condition for the TCP forwarding rule.
func NewForwardingRuleCondition(frName string) metav1.Condition {
	return NewConditionResourceAllocated(L4LBForwardingRuleConditionType, frName)
}

// NewHealthCheckCondition creates a condition for the health check.
func NewHealthCheckCondition(hcName string) metav1.Condition {
	return NewConditionResourceAllocated(L4LBHealthCheckConditionType, hcName)
}

// NewFirewallCondition creates a condition for the firewall.
func NewFirewallCondition(fwName string) metav1.Condition {
	return NewConditionResourceAllocated(L4LBFirewallRuleConditionType, fwName)
}

// NewFirewallHealthCheckCondition creates a condition for the firewall health check.
func NewFirewallHealthCheckCondition(fwName string) metav1.Condition {
	return NewConditionResourceAllocated(L4LBFirewallRuleHealthCheckConditionType, fwName)
}
