package service

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	servicehelper "k8s.io/cloud-provider/service/helpers"
)

// TestNeedsCleanup verifies that services with deletionTimestamp set are
// correctly identified as needing cleanup, regardless of finalizer state.
// This prevents a race condition where node updates trigger reconciliation
// after the finalizer is removed but before the service is deleted from etcd.
func TestNeedsCleanup(t *testing.T) {
	now := metav1.Now()

	testCases := []struct {
		name     string
		service  *v1.Service
		expected bool
	}{
		{
			name: "service with deletionTimestamp and finalizer should need cleanup",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
					Finalizers:        []string{servicehelper.LoadBalancerCleanupFinalizer},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			expected: true,
		},
		{
			name: "service with deletionTimestamp but no finalizer should need cleanup",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
					Finalizers:        []string{},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			expected: true,
		},
		{
			name: "service without deletionTimestamp and without finalizer should not need cleanup",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			expected: false,
		},
		{
			name: "service without deletionTimestamp but with finalizer should not need cleanup",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{servicehelper.LoadBalancerCleanupFinalizer},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			expected: false,
		},
		{
			name: "non-LoadBalancer service with finalizer should need cleanup",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{servicehelper.LoadBalancerCleanupFinalizer},
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := needsCleanup(tc.service)
			if result != tc.expected {
				t.Errorf("needsCleanup() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

// TestNodeSyncService_DeletionTimestamp verifies that nodeSyncService returns
// success (false) without attempting to sync when a service has deletionTimestamp set.
func TestNodeSyncService_DeletionTimestamp(t *testing.T) {
	now := metav1.Now()

	testCases := []struct {
		name    string
		service *v1.Service
	}{
		{
			name:    "nil service should return success",
			service: nil,
		},
		{
			name: "service with deletionTimestamp should return success without syncing",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
		},
		{
			name: "non-LoadBalancer service should return success",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Controller{}

			result := c.nodeSyncService(tc.service)
			if result != false {
				t.Errorf("nodeSyncService() returned needRetry=true, expected success (false)")
			}
		})
	}
}
