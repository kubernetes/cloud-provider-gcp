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
	"reflect"
	"strings"
	"testing"

	compute "google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	netutils "k8s.io/utils/net"
)

func TestLastIPInRange(t *testing.T) {
	for _, tc := range []struct {
		cidr string
		want string
	}{
		{"10.1.2.3/32", "10.1.2.3"},
		{"10.1.2.0/31", "10.1.2.1"},
		{"10.1.0.0/30", "10.1.0.3"},
		{"10.0.0.0/29", "10.0.0.7"},
		{"::0/128", "::"},
		{"::0/127", "::1"},
		{"::0/126", "::3"},
		{"::0/120", "::ff"},
	} {
		_, c, err := netutils.ParseCIDRSloppy(tc.cidr)
		if err != nil {
			t.Errorf("can't parse CIDR %v = _, %v, %v; want nil", tc.cidr, c, err)
			continue
		}

		if lastIP := lastIPInRange(c); lastIP.String() != tc.want {
			t.Errorf("LastIPInRange(%v) = %v; want %v", tc.cidr, lastIP, tc.want)
		}
	}
}

func TestSubnetsInCIDR(t *testing.T) {
	subnets := []*compute.Subnetwork{
		{
			Name:        "A",
			IpCidrRange: "10.0.0.0/20",
		},
		{
			Name:        "B",
			IpCidrRange: "10.0.16.0/20",
		},
		{
			Name:        "C",
			IpCidrRange: "10.132.0.0/20",
		},
		{
			Name:        "D",
			IpCidrRange: "10.0.32.0/20",
		},
		{
			Name:        "E",
			IpCidrRange: "10.134.0.0/20",
		},
	}
	expectedNames := []string{"C", "E"}

	gotSubs, err := subnetsInCIDR(subnets, autoSubnetIPRange)
	if err != nil {
		t.Errorf("autoSubnetInList() = _, %v", err)
	}

	var gotNames []string
	for _, v := range gotSubs {
		gotNames = append(gotNames, v.Name)
	}
	if !reflect.DeepEqual(gotNames, expectedNames) {
		t.Errorf("autoSubnetInList() = %v, expected: %v", gotNames, expectedNames)
	}
}

func TestFirewallToGcloudArgs(t *testing.T) {
	firewall := compute.Firewall{
		Description:  "Last Line of Defense",
		TargetTags:   []string{"jock-nodes", "band-nodes"},
		SourceRanges: []string{"3.3.3.3/20", "1.1.1.1/20", "2.2.2.2/20"},
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "udp",
				Ports:      []string{"321", "123-456", "123"},
			},
			{
				IPProtocol: "tcp",
				Ports:      []string{"321", "123-456", "123"},
			},
			{
				IPProtocol: "sctp",
				Ports:      []string{"321", "123-456", "123"},
			},
		},
	}
	got := firewallToGcloudArgs(&firewall, "my-project")

	var e = `--description "Last Line of Defense" --allow sctp:123,sctp:123-456,sctp:321,tcp:123,tcp:123-456,tcp:321,udp:123,udp:123-456,udp:321 --source-ranges 1.1.1.1/20,2.2.2.2/20,3.3.3.3/20 --target-tags band-nodes,jock-nodes --project my-project`
	if got != e {
		t.Errorf("%q does not equal %q", got, e)
	}
}

func TestFirewallToGCloudCreateCmd(t *testing.T) {
	testCases := []struct {
		desc      string
		fw        *compute.Firewall
		projectID string
		wantArgs  []string
	}{
		{
			desc: "pinhole rule with destination ranges",
			fw: &compute.Firewall{
				Name:              "k8s-fw-pinhole",
				Network:           "projects/test-project/global/networks/default",
				Description:       "pinhole rule",
				SourceRanges:      []string{"10.0.0.0/8"},
				DestinationRanges: []string{"192.168.1.2/32", "192.168.1.1/32"},
				TargetTags:        []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-pinhole",
				"--network default",
				`--description "pinhole rule"`,
				"--allow tcp:80",
				"--source-ranges 10.0.0.0/8",
				"--destination-ranges 192.168.1.1/32,192.168.1.2/32",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "standard rule without destination ranges",
			fw: &compute.Firewall{
				Name:         "k8s-fw-std",
				Network:      "projects/test-project/global/networks/default",
				Description:  "std rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-std",
				"--network default",
				`--description "std rule"`,
				"--allow tcp:80",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "ipv6 pinhole rule",
			fw: &compute.Firewall{
				Name:              "k8s-fw-ipv6-pinhole",
				Network:           "projects/test-project/global/networks/default",
				Description:       "ipv6 pinhole rule",
				SourceRanges:      []string{"2001:db8::/32"},
				DestinationRanges: []string{"2001:db8::1/128"},
				TargetTags:        []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-ipv6-pinhole",
				"--network default",
				`--description "ipv6 pinhole rule"`,
				"--allow tcp:80",
				"--source-ranges 2001:db8::/32",
				"--destination-ranges 2001:db8::1/128",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "deny rule with priority, direction, and disabled",
			fw: &compute.Firewall{
				Name:         "k8s-fw-deny",
				Network:      "projects/test-project/global/networks/default",
				Description:  "deny rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Denied: []*compute.FirewallDenied{
					{IPProtocol: "tcp", Ports: []string{"8080"}},
				},
				Priority:  500,
				Direction: "INGRESS",
				Disabled:  true,
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-deny",
				"--network default",
				`--description "deny rule"`,
				"--deny tcp:8080",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--priority 500",
				"--direction INGRESS",
				"--disabled",
				"--project test-project",
			},
		},
		{
			desc: "rule with multiple protocols and multiple ports",
			fw: &compute.Firewall{
				Name:         "k8s-fw-multi-port",
				Network:      "projects/test-project/global/networks/default",
				Description:  "multi port rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					{IPProtocol: "udp", Ports: []string{"53"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-multi-port",
				"--network default",
				`--description "multi port rule"`,
				"--allow tcp:443,tcp:80,udp:53",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "deny all rule without ports",
			fw: &compute.Firewall{
				Name:         "k8s-fw-deny-all",
				Network:      "projects/test-project/global/networks/default",
				Description:  "deny all rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Denied: []*compute.FirewallDenied{
					{IPProtocol: "all"},
				},
				Priority:  500,
				Direction: "INGRESS",
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-deny-all",
				"--network default",
				`--description "deny all rule"`,
				"--deny all",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--priority 500",
				"--direction INGRESS",
				"--project test-project",
			},
		},
		{
			desc: "rule allowing ICMP without ports",
			fw: &compute.Firewall{
				Name:         "k8s-fw-icmp",
				Network:      "projects/test-project/global/networks/default",
				Description:  "icmp rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "icmp"},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-icmp",
				"--network default",
				`--description "icmp rule"`,
				"--allow icmp",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "complex multi-protocol rule with tcp, udp, icmp, esp, and ah",
			fw: &compute.Firewall{
				Name:         "k8s-fw-complex",
				Network:      "projects/test-project/global/networks/default",
				Description:  "complex multi proto rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					{IPProtocol: "udp", Ports: []string{"53"}},
					{IPProtocol: "icmp"},
					{IPProtocol: "esp"},
					{IPProtocol: "ah"},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules create k8s-fw-complex",
				"--network default",
				`--description "complex multi proto rule"`,
				"--allow ah,esp,icmp,tcp:443,tcp:80,udp:53",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := FirewallToGCloudCreateCmd(tc.fw, tc.projectID)
			want := strings.Join(tc.wantArgs, " ")
			if got != want {
				t.Errorf("%s failed:\nGot:  %q\nWant: %q", tc.desc, got, want)
			}
		})
	}
}

func TestFirewallToGCloudUpdateCmd(t *testing.T) {
	testCases := []struct {
		desc      string
		fw        *compute.Firewall
		projectID string
		wantArgs  []string
	}{
		{
			desc: "pinhole rule with destination ranges",
			fw: &compute.Firewall{
				Name:              "k8s-fw-pinhole",
				Network:           "projects/test-project/global/networks/default",
				Description:       "pinhole rule",
				SourceRanges:      []string{"10.0.0.0/8"},
				DestinationRanges: []string{"192.168.1.2/32", "192.168.1.1/32"},
				TargetTags:        []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-pinhole",
				`--description "pinhole rule"`,
				"--allow tcp:80",
				"--source-ranges 10.0.0.0/8",
				"--destination-ranges 192.168.1.1/32,192.168.1.2/32",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "standard rule without destination ranges",
			fw: &compute.Firewall{
				Name:         "k8s-fw-std",
				Network:      "projects/test-project/global/networks/default",
				Description:  "std rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-std",
				`--description "std rule"`,
				"--allow tcp:80",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "ipv6 pinhole rule",
			fw: &compute.Firewall{
				Name:              "k8s-fw-ipv6-pinhole",
				Network:           "projects/test-project/global/networks/default",
				Description:       "ipv6 pinhole rule",
				SourceRanges:      []string{"2001:db8::/32"},
				DestinationRanges: []string{"2001:db8::1/128"},
				TargetTags:        []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-ipv6-pinhole",
				`--description "ipv6 pinhole rule"`,
				"--allow tcp:80",
				"--source-ranges 2001:db8::/32",
				"--destination-ranges 2001:db8::1/128",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "deny rule with priority, direction, and disabled",
			fw: &compute.Firewall{
				Name:         "k8s-fw-deny",
				Network:      "projects/test-project/global/networks/default",
				Description:  "deny rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Denied: []*compute.FirewallDenied{
					{IPProtocol: "tcp", Ports: []string{"8080"}},
				},
				Priority:  500,
				Direction: "INGRESS",
				Disabled:  true,
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-deny",
				`--description "deny rule"`,
				"--deny tcp:8080",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--priority 500",
				"--direction INGRESS",
				"--disabled",
				"--project test-project",
			},
		},
		{
			desc: "rule with multiple protocols and multiple ports",
			fw: &compute.Firewall{
				Name:         "k8s-fw-multi-port",
				Network:      "projects/test-project/global/networks/default",
				Description:  "multi port rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					{IPProtocol: "udp", Ports: []string{"53"}},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-multi-port",
				`--description "multi port rule"`,
				"--allow tcp:443,tcp:80,udp:53",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "deny all rule without ports",
			fw: &compute.Firewall{
				Name:         "k8s-fw-deny-all",
				Network:      "projects/test-project/global/networks/default",
				Description:  "deny all rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Denied: []*compute.FirewallDenied{
					{IPProtocol: "all"},
				},
				Priority:  500,
				Direction: "INGRESS",
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-deny-all",
				`--description "deny all rule"`,
				"--deny all",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--priority 500",
				"--direction INGRESS",
				"--project test-project",
			},
		},
		{
			desc: "rule allowing ICMP without ports",
			fw: &compute.Firewall{
				Name:         "k8s-fw-icmp",
				Network:      "projects/test-project/global/networks/default",
				Description:  "icmp rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "icmp"},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-icmp",
				`--description "icmp rule"`,
				"--allow icmp",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
		{
			desc: "complex multi-protocol rule with tcp, udp, icmp, esp, and ah",
			fw: &compute.Firewall{
				Name:         "k8s-fw-complex",
				Network:      "projects/test-project/global/networks/default",
				Description:  "complex multi proto rule",
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"node-tag"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					{IPProtocol: "udp", Ports: []string{"53"}},
					{IPProtocol: "icmp"},
					{IPProtocol: "esp"},
					{IPProtocol: "ah"},
				},
			},
			projectID: "test-project",
			wantArgs: []string{
				"gcloud compute firewall-rules update k8s-fw-complex",
				`--description "complex multi proto rule"`,
				"--allow ah,esp,icmp,tcp:443,tcp:80,udp:53",
				"--source-ranges 10.0.0.0/8",
				"--target-tags node-tag",
				"--project test-project",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := FirewallToGCloudUpdateCmd(tc.fw, tc.projectID)
			want := strings.Join(tc.wantArgs, " ")
			if got != want {
				t.Errorf("%s failed:\nGot:  %q\nWant: %q", tc.desc, got, want)
			}
		})
	}
}

func TestFirewallToGCloud_FieldExhaustiveness(t *testing.T) {
	knownFields := map[string]bool{
		"Allowed":               true,
		"CreationTimestamp":     false,
		"Denied":                true,
		"Description":           true,
		"DestinationRanges":     true,
		"Direction":             true,
		"Disabled":              true,
		"Id":                    false,
		"Kind":                  false,
		"LogConfig":             false,
		"Name":                  true,
		"Network":               true,
		"Params":                false,
		"Priority":              true,
		"SelfLink":              false,
		"SourceRanges":          true,
		"SourceServiceAccounts": false,
		"SourceTags":            false,
		"TargetServiceAccounts": false,
		"TargetTags":            true,
		"ServerResponse":        false,
		"ForceSendFields":       false,
		"NullFields":            false,
	}

	for field := range reflect.TypeOf(compute.Firewall{}).Fields() {
		if _, evaluated := knownFields[field.Name]; !evaluated {
			t.Fatalf(
				"New field %q added to compute.Firewall!\n"+
					"Evaluate whether it needs gcloud flag mapping in\n"+
					"FirewallToGCloudCreateCmd / FirewallToGCloudUpdateCmd,\n"+
					"then update knownFields in this test.",
				field.Name,
			)
		}
	}
}

// TestAddRemoveFinalizer tests the add/remove and hasFinalizer methods.
func TestAddRemoveFinalizer(t *testing.T) {
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	gce, err := fakeGCECloud(vals)
	if err != nil {
		t.Fatalf("Failed to get GCE client, err %v", err)
	}
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Failed to create service %s, err %v", svc.Name, err)
	}

	err = addFinalizer(svc, gce.client.CoreV1(), ILBFinalizerV1)
	if err != nil {
		t.Fatalf("Failed to add finalizer, err %v", err)
	}
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get service, err %v", err)
	}
	if !hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Unable to find finalizer '%s' in service %s", ILBFinalizerV1, svc.Name)
	}
	err = removeFinalizer(svc, gce.client.CoreV1(), ILBFinalizerV1)
	if err != nil {
		t.Fatalf("Failed to remove finalizer, err %v", err)
	}
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get service, err %v", err)
	}
	if hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Failed to remove finalizer '%s' in service %s", ILBFinalizerV1, svc.Name)
	}
}
