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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
)

func TestGetLoadBalancer(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)

	// When a loadbalancer has not been created
	status, found, err := gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
	assert.Nil(t, status)
	assert.False(t, found)
	assert.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	expectedStatus, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	require.NoError(t, err)

	status, found, err = gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
	assert.Equal(t, expectedStatus, status)
	assert.True(t, found)
	assert.NoError(t, err)

	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, apiService)
	require.NoError(t, err)

	status, found, err = gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
	assert.Nil(t, status)
	assert.False(t, found)
	assert.NoError(t, err)

	apiService.Finalizers = []string{NetLBFinalizerV1}
	status, found, err = gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
	assert.Equal(t, &v1.LoadBalancerStatus{}, status)
	assert.True(t, found)
	assert.NoError(t, err)

	apiService.Finalizers = []string{NetLBFinalizerV1}
	status, found, err = gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
	assert.Equal(t, &v1.LoadBalancerStatus{}, status)
	assert.True(t, found)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancerCreatesExternalLb(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)

	status, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	assertExternalLbResources(t, gce, apiService, vals, nodeNames)
}

func TestEnsureLoadBalancerCreatesInternalLb(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService(string(LBTypeInternal))
	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	assertInternalLbResources(t, gce, apiService, vals, nodeNames)
}

func TestEnsureLoadBalancerDeletesExistingInternalLb(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)

	createInternalLoadBalancer(gce, apiService, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)

	status, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)

	assertExternalLbResources(t, gce, apiService, vals, nodeNames)
	assertInternalLbResourcesDeleted(t, gce, apiService, vals, false)
}

func TestEnsureLoadBalancerDeletesExistingExternalLb(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)

	createExternalLoadBalancer(gce, apiService, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)

	apiService = fakeLoadbalancerService(string(LBTypeInternal))

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Update(context.TODO(), apiService, metav1.UpdateOptions{})
	require.NoError(t, err)

	status, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)

	assertInternalLbResources(t, gce, apiService, vals, nodeNames)
	assertExternalLbResourcesDeleted(t, gce, apiService, vals, false)
}

func TestEnsureLoadBalancerDeletedDeletesExternalLb(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	_, err = createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)

	createExternalLoadBalancer(gce, apiService, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)

	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Get(context.TODO(), apiService.Name, metav1.GetOptions{})
	require.NoError(t, err)

	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, apiService)
	assert.NoError(t, err)
	assertExternalLbResourcesDeleted(t, gce, apiService, vals, true)
}

func TestEnsureLoadBalancerDeletedDeletesInternalLb(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	_, err = createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService(string(LBTypeInternal))
	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)
	createInternalLoadBalancer(gce, apiService, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)

	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, apiService)
	assert.NoError(t, err)
	assertInternalLbResourcesDeleted(t, gce, apiService, vals, true)
}

func TestProjectsBasePath(t *testing.T) {
	t.Parallel()
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	// Loadbalancer controller code expects basepath to contain the projects string.
	expectProjectsBasePath := "https://compute.googleapis.com/compute/v1/projects/"
	// See https://github.com/kubernetes/kubernetes/issues/102757, the endpoint can have mtls in some cases.
	expectMtlsProjectsBasePath := "https://compute.mtls.googleapis.com/compute/v1/projects/"
	require.NoError(t, err)
	if gce.projectsBasePath != expectProjectsBasePath && gce.projectsBasePath != expectMtlsProjectsBasePath {
		t.Errorf("Compute projectsBasePath has changed. Got %q, want %q or %q", gce.projectsBasePath, expectProjectsBasePath, expectMtlsProjectsBasePath)
	}
}

func TestEnsureLoadBalancerMixedProtocols(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")
	apiService.Spec.Ports = append(apiService.Spec.Ports, v1.ServicePort{
		Protocol: v1.ProtocolUDP,
		Port:     int32(8080),
	})
	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	if err == nil {
		t.Errorf("Expected error ensuring loadbalancer for Service with multiple ports")
	}
	if err.Error() != "mixed protocol is not supported for LoadBalancer" {
		t.Fatalf("unexpected error, got: %s wanted \"mixed protocol is not supported for LoadBalancer\"", err.Error())
	}
	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Get(context.TODO(), apiService.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !hasLoadBalancerPortsError(apiService) {
		t.Fatalf("Expected condition %v to be True, got %v", v1.LoadBalancerPortsError, apiService.Status.Conditions)
	}
}

func TestUpdateLoadBalancerMixedProtocols(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	apiService := fakeLoadbalancerService("")
	apiService.Spec.Ports = append(apiService.Spec.Ports, v1.ServicePort{
		Protocol: v1.ProtocolUDP,
		Port:     int32(8080),
	})
	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
	require.NoError(t, err)

	// create an external loadbalancer to simulate an upgrade scenario where the loadbalancer exists
	// before the new controller is running and later the Service is updated
	_, err = createExternalLoadBalancer(gce, apiService, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	err = gce.UpdateLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Get(context.TODO(), apiService.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !hasLoadBalancerPortsError(apiService) {
		t.Fatalf("Expected condition %v to be True, got %v", v1.LoadBalancerPortsError, apiService.Status.Conditions)
	}
}

func TestCheckMixedProtocol(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		ports       []v1.ServicePort
		wantErr     error
	}{
		{
			name:        "TCP",
			annotations: make(map[string]string),
			ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     int32(8080),
				},
			},
			wantErr: nil,
		},
		{
			name:        "UDP",
			annotations: map[string]string{ServiceAnnotationLoadBalancerType: "nlb"},
			ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolUDP,
					Port:     int32(8080),
				},
			},
			wantErr: nil,
		},
		{
			name:        "2 TCP",
			annotations: make(map[string]string),
			ports: []v1.ServicePort{
				{
					Name:     "port80",
					Protocol: v1.ProtocolTCP,
					Port:     int32(80),
				},
				{
					Name:     "port8080",
					Protocol: v1.ProtocolTCP,
					Port:     int32(8080),
				},
			},
			wantErr: nil,
		},
		{
			name:        "2 UDP",
			annotations: map[string]string{ServiceAnnotationLoadBalancerType: "nlb"},
			ports: []v1.ServicePort{
				{
					Name:     "port80",
					Protocol: v1.ProtocolUDP,
					Port:     int32(80),
				},
				{
					Name:     "port8080",
					Protocol: v1.ProtocolUDP,
					Port:     int32(8080),
				},
			},
			wantErr: nil,
		},
		{
			name:        "TCP and UDP",
			annotations: map[string]string{ServiceAnnotationLoadBalancerType: "nlb"},
			ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolUDP,
					Port:     int32(53),
				},
				{
					Protocol: v1.ProtocolTCP,
					Port:     int32(53),
				},
			},
			wantErr: fmt.Errorf("mixed protocol is not supported for LoadBalancer"),
		},
	}
	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkMixedProtocol(tt.ports)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.Equal(t, err, nil)
			}
		})
	}
}

func Test_hasLoadBalancerPortsError(t *testing.T) {
	tests := []struct {
		name    string
		service *v1.Service
		want    bool
	}{
		{
			name:    "no status",
			service: &v1.Service{},
		},
		{
			name: "condition set to true",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "service1"},
				Spec: v1.ServiceSpec{
					ClusterIPs: []string{"1.2.3.4"},
					Type:       "LoadBalancer",
					Ports:      []v1.ServicePort{{Port: 80, Protocol: "TCP"}},
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{{IP: "2.3.4.5"}, {IP: "3.4.5.6"}}},
					Conditions: []metav1.Condition{
						{
							Type:   v1.LoadBalancerPortsError,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "condition set false",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "service1"},
				Spec: v1.ServiceSpec{
					ClusterIPs: []string{"1.2.3.4"},
					Type:       "LoadBalancer",
					Ports:      []v1.ServicePort{{Port: 80, Protocol: "TCP"}},
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{{IP: "2.3.4.5"}, {IP: "3.4.5.6"}}},
					Conditions: []metav1.Condition{
						{
							Type:   v1.LoadBalancerPortsError,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
		},
		{
			name: "multiple conditions unrelated",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "service1"},
				Spec: v1.ServiceSpec{
					ClusterIPs: []string{"1.2.3.4"},
					Type:       "LoadBalancer",
					Ports:      []v1.ServicePort{{Port: 80, Protocol: "TCP"}},
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{{IP: "2.3.4.5"}, {IP: "3.4.5.6"}}},
					Conditions: []metav1.Condition{
						{
							Type:   "condition1",
							Status: metav1.ConditionFalse,
						},
						{
							Type:   "condition2",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasLoadBalancerPortsError(tt.service); got != tt.want {
				t.Errorf("hasLoadBalancerPortsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureLoadBalancerServiceWithLoadBalancerClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc              string
		loadBalancerClass string
		shouldProcess     bool
	}{
		{
			desc:              "Custom loadBalancerClass should not process",
			loadBalancerClass: "customLBClass",
			shouldProcess:     false,
		},
		{
			desc:              "Use legacy ILB loadBalancerClass",
			loadBalancerClass: LegacyRegionalInternalLoadBalancerClass,
			shouldProcess:     true,
		},
		{
			desc:              "Use legacy NetLB loadBalancerClass",
			loadBalancerClass: LegacyRegionalExternalLoadBalancerClass,
			shouldProcess:     true,
		},
		{
			desc:              "Unset loadBalancerClass",
			loadBalancerClass: "",
			shouldProcess:     true,
		},
	} {

		vals := DefaultTestClusterValues()
		gce, err := fakeGCECloud(vals)
		require.NoError(t, err)

		nodeNames := []string{"test-node-1"}
		nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
		require.NoError(t, err)

		apiService := fakeLoadbalancerServiceWithLoadBalancerClass("", tc.loadBalancerClass)
		if tc.loadBalancerClass == "" {
			apiService = fakeLoadbalancerService("")
		}

		apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
		assert.NoError(t, err)
		expectedStatus, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)

		if tc.shouldProcess {
			require.NoError(t, err)
			status, found, err := gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
			assert.Equal(t, expectedStatus, status)
			assert.True(t, found)
			assert.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
			assert.Empty(t, expectedStatus)
		}
	}
}

func TestUpdateLoadBalancerWithLoadBalancerClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc              string
		loadBalancerClass string
		shouldProcess     bool
	}{
		{
			desc:              "Update with custom loadBalancerClass, should not process",
			loadBalancerClass: "customLBClass",
			shouldProcess:     false,
		},
		{
			desc:              "Update with legacy ILB loadBalancerClass",
			loadBalancerClass: LegacyRegionalInternalLoadBalancerClass,
			shouldProcess:     true,
		},
		{
			desc:              "Update with legacy NetLB loadBalancerClass",
			loadBalancerClass: LegacyRegionalExternalLoadBalancerClass,
			shouldProcess:     true,
		},
		{
			desc:              "Update with loadBalancerClass unset",
			loadBalancerClass: "",
			shouldProcess:     true,
		},
	} {
		vals := DefaultTestClusterValues()
		gce, err := fakeGCECloud(vals)
		require.NoError(t, err)

		nodeNames := []string{"test-node-1"}
		nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
		require.NoError(t, err)

		apiService := fakeLoadbalancerServiceWithLoadBalancerClass("", tc.loadBalancerClass)
		if tc.loadBalancerClass == "" {
			apiService = fakeLoadbalancerService("")
		}

		apiService, err = gce.client.CoreV1().Services(apiService.Namespace).Create(context.TODO(), apiService, metav1.CreateOptions{})
		assert.NoError(t, err)
		gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)

		apiService.Spec.Ports = append(apiService.Spec.Ports, v1.ServicePort{
			Protocol: v1.ProtocolTCP,
			Port:     int32(80),
		})
		err = gce.UpdateLoadBalancer(context.Background(), vals.ClusterName, apiService, nodes)

		if tc.shouldProcess {
			require.NoError(t, err)
			_, found, err := gce.GetLoadBalancer(context.Background(), vals.ClusterName, apiService)
			assert.True(t, found)
			assert.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
		}

		err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, apiService)
		if tc.shouldProcess {
			assert.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
		}
	}
}

func TestConditionsEqual(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	later := metav1.NewTime(now.Time.Add(1 * time.Hour))

	condReady := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "ReadyReason",
		Message:            "ReadyMessage",
		LastTransitionTime: now,
		ObservedGeneration: 1,
	}

	condReadyDiffStatus := condReady
	condReadyDiffStatus.Status = metav1.ConditionFalse

	condReadyDiffTime := condReady
	condReadyDiffTime.LastTransitionTime = later

	condError := metav1.Condition{
		Type:               "Error",
		Status:             metav1.ConditionTrue,
		Reason:             "ErrorReason",
		Message:            "ErrorMessage",
		LastTransitionTime: now,
		ObservedGeneration: 1,
	}

	tests := []struct {
		name    string
		current []metav1.Condition
		desired []metav1.Condition
		want    bool
	}{
		{
			name:    "both nil",
			current: nil,
			desired: nil,
			want:    true,
		},
		{
			name:    "both empty",
			current: []metav1.Condition{},
			desired: []metav1.Condition{},
			want:    true,
		},
		{
			name:    "nil and empty",
			current: nil,
			desired: []metav1.Condition{},
			want:    true,
		},
		{
			name:    "len mismatch",
			current: []metav1.Condition{condReady},
			desired: []metav1.Condition{condReady, condError},
			want:    false,
		},
		{
			name:    "exact match single",
			current: []metav1.Condition{condReady},
			desired: []metav1.Condition{condReady},
			want:    true,
		},
		{
			name:    "exact match multiple",
			current: []metav1.Condition{condReady, condError},
			desired: []metav1.Condition{condReady, condError},
			want:    true,
		},
		{
			name:    "order mismatch but content equal",
			current: []metav1.Condition{condReady, condError},
			desired: []metav1.Condition{condError, condReady},
			want:    true,
		},
		{
			name:    "same type different status",
			current: []metav1.Condition{condReady},
			desired: []metav1.Condition{condReadyDiffStatus},
			want:    false,
		},
		{
			name:    "same type different time",
			current: []metav1.Condition{condReady},
			desired: []metav1.Condition{condReadyDiffTime},
			want:    false,
		},
		{
			name:    "different types same length",
			current: []metav1.Condition{condReady},
			desired: []metav1.Condition{condError},
			want:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ConditionsEqual(tt.current, tt.desired)
			assert.Equal(t, tt.want, got)
		})
	}
}
