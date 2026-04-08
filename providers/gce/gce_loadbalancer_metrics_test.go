//go:build !providerless
// +build !providerless

/*
Copyright 2020 The Kubernetes Authors.

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
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/component-base/metrics/testutil"
)

func TestComputeL4ILBMetrics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc             string
		serviceStates    []L4ILBServiceState
		expectL4ILBCount map[feature]int
	}{
		{
			desc:          "empty input",
			serviceStates: []L4ILBServiceState{},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      0,
				l4ILBGlobalAccess: 0,
				l4ILBCustomSubnet: 0,
				l4ILBInSuccess:    0,
				l4ILBInError:      0,
			},
		},
		{
			desc: "one l4 ilb service",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(false, false, true),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      1,
				l4ILBGlobalAccess: 0,
				l4ILBCustomSubnet: 0,
				l4ILBInSuccess:    1,
				l4ILBInError:      0,
			},
		},
		{
			desc: "l4 ilb service in error state",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(false, true, false),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      1,
				l4ILBGlobalAccess: 0,
				l4ILBCustomSubnet: 0,
				l4ILBInSuccess:    0,
				l4ILBInError:      1,
			},
		},
		{
			desc: "global access for l4 ilb service enabled",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(true, false, true),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      1,
				l4ILBGlobalAccess: 1,
				l4ILBCustomSubnet: 0,
				l4ILBInSuccess:    1,
				l4ILBInError:      0,
			},
		},
		{
			desc: "custom subnet for l4 ilb service enabled",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(false, true, true),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      1,
				l4ILBGlobalAccess: 0,
				l4ILBCustomSubnet: 1,
				l4ILBInSuccess:    1,
				l4ILBInError:      0,
			},
		},
		{
			desc: "both global access and custom subnet for l4 ilb service enabled",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(true, true, true),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      1,
				l4ILBGlobalAccess: 1,
				l4ILBCustomSubnet: 1,
				l4ILBInSuccess:    1,
				l4ILBInError:      0,
			},
		},
		{
			desc: "many l4 ilb services",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(false, false, true),
				newL4ILBServiceState(false, true, true),
				newL4ILBServiceState(true, false, true),
				newL4ILBServiceState(true, true, true),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      4,
				l4ILBGlobalAccess: 2,
				l4ILBCustomSubnet: 2,
				l4ILBInSuccess:    4,
				l4ILBInError:      0,
			},
		},
		{
			desc: "many l4 ilb services with some in error state",
			serviceStates: []L4ILBServiceState{
				newL4ILBServiceState(false, false, true),
				newL4ILBServiceState(false, true, false),
				newL4ILBServiceState(false, true, true),
				newL4ILBServiceState(true, false, true),
				newL4ILBServiceState(true, false, false),
				newL4ILBServiceState(true, true, true),
			},
			expectL4ILBCount: map[feature]int{
				l4ILBService:      6,
				l4ILBGlobalAccess: 2,
				l4ILBCustomSubnet: 2,
				l4ILBInSuccess:    4,
				l4ILBInError:      2,
			},
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			newMetrics := LoadBalancerMetrics{
				l4ILBServiceMap: make(map[string]L4ILBServiceState),
			}
			for i, serviceState := range tc.serviceStates {
				newMetrics.SetL4ILBService(strconv.Itoa(i), serviceState)
			}
			got := newMetrics.computeL4ILBMetrics()
			if diff := cmp.Diff(tc.expectL4ILBCount, got); diff != "" {
				t.Fatalf("Got diff for L4 ILB service counts (-want +got):\n%s", diff)
			}
		})
	}
}

func newL4ILBServiceState(globalAccess, customSubnet, inSuccess bool) L4ILBServiceState {
	return L4ILBServiceState{
		EnabledGlobalAccess: globalAccess,
		EnabledCustomSubnet: customSubnet,
		InSuccess:           inSuccess,
	}
}

func TestL4NetLBMetrics(t *testing.T) {
	metrics := newLoadBalancerMetrics()
	// Cast to *LoadBalancerMetrics to access methods
	lbMetrics, ok := metrics.(*LoadBalancerMetrics)
	if !ok {
		t.Fatalf("Failed to cast loadbalancerMetricsCollector to *LoadBalancerMetrics")
	}

	lbMetrics.SetL4NetLBService("svc-success-ipv4", L4NetLBServiceState{
		Status:       StatusSuccess,
		DenyFirewall: DenyFirewallStatusIPv4,
	})
	lbMetrics.SetL4NetLBService("svc-success-ipv4-2", L4NetLBServiceState{
		Status:       StatusSuccess,
		DenyFirewall: DenyFirewallStatusIPv4,
	})
	lbMetrics.SetL4NetLBService("svc-success-disabled", L4NetLBServiceState{
		Status:       StatusSuccess,
		DenyFirewall: DenyFirewallStatusDisabled,
	})
	lbMetrics.SetL4NetLBService("svc-error-none", L4NetLBServiceState{
		Status:       StatusError,
		DenyFirewall: DenyFirewallStatusNone,
	})
	lbMetrics.SetL4NetLBService("svc-user-error-none", L4NetLBServiceState{
		Status:       StatusUserError,
		DenyFirewall: DenyFirewallStatusNone,
	})
	lbMetrics.SetL4NetLBService("svc-persistent-error-none", L4NetLBServiceState{
		Status:       StatusPersistentError,
		DenyFirewall: DenyFirewallStatusNone,
	})

	// Add keys to be checked for deletion
	lbMetrics.SetL4NetLBService("svc-to-delete", L4NetLBServiceState{
		Status:       StatusSuccess,
		DenyFirewall: DenyFirewallStatusNone,
	})
	lbMetrics.DeleteL4NetLBService("svc-to-delete")

	lbMetrics.exportNetLBMetrics()

	verifyL4NetLBMetric(t, 2, StatusSuccess, DenyFirewallStatusIPv4)
	verifyL4NetLBMetric(t, 1, StatusSuccess, DenyFirewallStatusDisabled)
	verifyL4NetLBMetric(t, 1, StatusError, DenyFirewallStatusNone)
	verifyL4NetLBMetric(t, 1, StatusUserError, DenyFirewallStatusNone)
	verifyL4NetLBMetric(t, 1, StatusPersistentError, DenyFirewallStatusNone)
}

func verifyL4NetLBMetric(t *testing.T, expectedCount int, status L4ServiceStatus, denyFirewall DenyFirewallStatus) {
	t.Helper()
	val, err := testutil.GetGaugeMetricValue(l4NetLBCount.WithLabelValues(string(status), string(denyFirewall)))
	if err != nil {
		t.Errorf("Failed to get metric value: %v", err)
	}
	if int(val) != expectedCount {
		t.Errorf("Expected count %d but got %d for status %s, denyFirewall %s", expectedCount, int(val), status, denyFirewall)
	}
}
