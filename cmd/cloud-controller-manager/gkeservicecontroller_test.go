package main

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	gkeservicecontroller "k8s.io/cloud-provider-gcp/pkg/controller/service"
)

// TestWantsLoadBalancer verifies that the forked and modified WantsLoadBalancer
// function in k8s.io/cloud-provider-gcp/pkg/controller/service has the correct
// logic. This test acts as a safeguard to prevent future updates from
// overwriting the custom logic. It ensures that the controller correctly
// processes services with GKE-specific legacy LoadBalancerClasses and,
// just as importantly, ignores services that have no LoadBalancerClass set
// or have a class that is not managed by this controller.
func TestWantsLoadBalancer(t *testing.T) {
	gkeLegacyExternalClass := "networking.gke.io/l4-regional-external-legacy"
	gkeLegacyInternalClass := "networking.gke.io/l4-regional-internal-legacy"
	otherClass := "some-other-class"

	testCases := []struct {
		name    string
		service *v1.Service
		want    bool
	}{
		{
			name: "service is not of type LoadBalancer",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
				},
			},
			want: false,
		},
		{
			name: "service is of type LoadBalancer with no loadBalancerClass",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeLoadBalancer,
				},
			},
			want: false,
		},
		{
			name: "service is of type LoadBalancer with GKE legacy external loadBalancerClass",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:              v1.ServiceTypeLoadBalancer,
					LoadBalancerClass: &gkeLegacyExternalClass,
				},
			},
			want: true,
		},
		{
			name: "service is of type LoadBalancer with GKE legacy internal loadBalancerClass",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:              v1.ServiceTypeLoadBalancer,
					LoadBalancerClass: &gkeLegacyInternalClass,
				},
			},
			want: true,
		},
		{
			name: "service is of type LoadBalancer with other loadBalancerClass",
			service: &v1.Service{
				Spec: v1.ServiceSpec{
					Type:              v1.ServiceTypeLoadBalancer,
					LoadBalancerClass: &otherClass,
				},
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gkeservicecontroller.WantsLoadBalancer(tc.service); got != tc.want {
				t.Errorf("WantsLoadBalancer() = %v, want %v", got, tc.want)
			}
		})
	}
}
