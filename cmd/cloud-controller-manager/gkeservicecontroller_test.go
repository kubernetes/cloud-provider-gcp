package main

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	gkeservicecontroller "k8s.io/cloud-provider-gcp/pkg/controller/service"
)

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
			want: true,
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
