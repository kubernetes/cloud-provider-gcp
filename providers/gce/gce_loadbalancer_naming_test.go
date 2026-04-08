//go:build !providerless
// +build !providerless

/*
Copyright 2025 The Kubernetes Authors.

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
package gce_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/cloud-provider-gcp/providers/gce"
)

func TestLoadBalancerNames(t *testing.T) {
	t.Parallel()
	type names struct {
		FirewallName     string
		DenyFirewallName string
	}

	testCases := []struct {
		desc string
		svc  *v1.Service
		want names
	}{
		{
			desc: "short_uid",
			svc:  &v1.Service{ObjectMeta: metav1.ObjectMeta{UID: "shortuidwith19chars"}},
			want: names{
				FirewallName:     "k8s-fw-ashortuidwith19chars",
				DenyFirewallName: "k8s-fw-ashortuidwith19chars-deny",
			},
		},
		{
			desc: "long_uid",
			svc:  &v1.Service{ObjectMeta: metav1.ObjectMeta{UID: "nextremelylonguidwithmorethan32charsthatwillbecutbecauseofaws32charlimitforloadbalancernames"}},
			want: names{
				FirewallName:     "k8s-fw-anextremelylonguidwithmorethan32",
				DenyFirewallName: "k8s-fw-anextremelylonguidwithmorethan32-deny",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			lbName := cloudprovider.DefaultLoadBalancerName(tC.svc)

			got := names{
				FirewallName:     gce.MakeFirewallName(lbName),
				DenyFirewallName: gce.MakeFirewallDenyName(lbName),
			}
			if diff := cmp.Diff(tC.want, got); diff != "" {
				t.Errorf("got != want, (-want, +got):/n%s", diff)
			}

			// https://docs.cloud.google.com/compute/docs/naming-resources#resource-name-format
			const gcpResourceNameLengthUpperLimit = 63
			v := reflect.ValueOf(got)
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				if len(f.String()) > gcpResourceNameLengthUpperLimit || len(f.String()) < 1 {
					t.Errorf("unacceptable length of resource name %q in field %q", f.String(), v.Type().Field(i).Name)
				}
			}
		})
	}
}
