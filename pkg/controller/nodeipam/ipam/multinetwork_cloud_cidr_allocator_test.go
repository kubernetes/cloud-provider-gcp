package ipam

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	compute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	networkv1alpha1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1alpha1"
	fake "k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/fake"
	networkinformers "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions"
)

const (
	group                = "networking.gke.io"
	gkeNetworkParamsKind = "GKENetworkParams"
	// Default Network
	defaultGKENetworkParamsName = "DefaultGKENetworkParams"
	defaultVPCName              = "projects/testProject/global/networks/default"
	defaultVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/default"
	defaultSecondaryRangeA      = "RangeA"
	defaultSecondaryRangeB      = "RangeB"
	// Red Network
	redNetworkName          = "Red-Network"
	redGKENetworkParamsName = "RedGKENetworkParams"
	redVPCName              = "projects/testProject/global/networks/red"
	redVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/red"
	redSecondaryRangeA      = "RedRangeA"
	redSecondaryRangeB      = "RedRangeB"
	// Blue Network
	blueNetworkName          = "Blue-Network"
	blueGKENetworkParamsName = "BlueGKENetworkParams"
	blueVPCName              = "projects/testProject/global/networks/blue"
	blueVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/blue"
	blueSecondaryRangeA      = "BlueRangeA"
	blueSecondaryRangeB      = "BlueRangeB"
)

func network(name, gkeNetworkParamsName string) *networkv1.Network {
	return &networkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1.NetworkSpec{
			Type: "L3",
			ParametersRef: &networkv1.NetworkParametersReference{
				Group: group,
				Kind:  gkeNetworkParamsKind,
				Name:  gkeNetworkParamsName,
			},
		},
	}
}

func gkeNetworkParams(name, vpc, subnet string, secRangeNames []string) *networkv1alpha1.GKENetworkParamSet {
	gnp := &networkv1alpha1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1alpha1.GKENetworkParamSetSpec{
			VPC:       vpc,
			VPCSubnet: subnet,
		},
	}
	if len(secRangeNames) > 0 {
		gnp.Spec.PodIPv4Ranges = &networkv1alpha1.SecondaryRanges{
			RangeNames: secRangeNames,
		}
	}
	return gnp
}

func interfaces(network, subnetwork, networkIP string, aliasIPRanges []*compute.AliasIpRange) *compute.NetworkInterface {
	return &compute.NetworkInterface{
		AliasIpRanges: aliasIPRanges,
		Network:       network,
		Subnetwork:    subnetwork,
		NetworkIP:     networkIP,
	}
}

func TestPerformMultiNetworkCIDRAllocation(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node0"},
		Spec:       v1.NodeSpec{ProviderID: ""},
	}
	testCases := []struct {
		desc                       string
		networks                   []*networkv1.Network
		gkeNwParams                []*networkv1alpha1.GKENetworkParamSet
		interfaces                 []*compute.NetworkInterface
		wantDefaultNwPodCIDRs      []string
		wantNorthInterfaces        networkv1.NorthInterfacesAnnotation
		wantAdditionalNodeNetworks networkv1.MultiNetworkAnnotation
		expectErr                  bool
	}{
		{
			desc: "default network only - should return default network cidrs and no multi-network annotations",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName),
			},
			gkeNwParams: []*networkv1alpha1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
			},
			interfaces: []*compute.NetworkInterface{
				interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
				}),
			},
			wantDefaultNwPodCIDRs: []string{"10.11.1.0/24"},
		},
		{
			desc: "one additional network along with default network",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName),
				network(redNetworkName, redGKENetworkParamsName),
			},
			gkeNwParams: []*networkv1alpha1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			interfaces: []*compute.NetworkInterface{
				interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
				}),
				interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
					{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
				}),
			},
			wantDefaultNwPodCIDRs: []string{"10.11.1.0/24"},
			wantNorthInterfaces: networkv1.NorthInterfacesAnnotation{
				{
					Network:   redNetworkName,
					IpAddress: "10.1.1.1",
				},
			},
			wantAdditionalNodeNetworks: networkv1.MultiNetworkAnnotation{
				{
					Name:  redNetworkName,
					Scope: "host-local",
					Cidrs: []string{"172.11.1.0/24"},
				},
			},
		},
		{
			desc: "no secondary ranges in GKENetworkParams",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName),
				network(redNetworkName, redGKENetworkParamsName),
				network(blueNetworkName, blueGKENetworkParamsName),
			},
			gkeNwParams: []*networkv1alpha1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{}),
			},
			interfaces: []*compute.NetworkInterface{
				interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
				}),
				interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
					{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
				}),
				interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
					{IpCidrRange: "20.28.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
				}),
			},
			wantDefaultNwPodCIDRs: []string{"10.11.1.0/24"},
			wantNorthInterfaces: networkv1.NorthInterfacesAnnotation{
				{
					Network:   redNetworkName,
					IpAddress: "10.1.1.1",
				},
				{
					Network:   blueNetworkName,
					IpAddress: "84.1.2.1",
				},
			},
			wantAdditionalNodeNetworks: networkv1.MultiNetworkAnnotation{
				{
					Name:  redNetworkName,
					Scope: "host-local",
					Cidrs: []string{"172.11.1.0/24"},
				},
			},
		},
		{
			desc: "networks without matching interfaces should be ignored",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName),
				network(redNetworkName, redGKENetworkParamsName),
				network(blueNetworkName, blueGKENetworkParamsName),
			},
			gkeNwParams: []*networkv1alpha1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{}),
			},
			interfaces: []*compute.NetworkInterface{
				interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
				}),
				interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
					{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
				}),
			},
			wantDefaultNwPodCIDRs: []string{"10.11.1.0/24"},
			wantNorthInterfaces: networkv1.NorthInterfacesAnnotation{
				{
					Network:   redNetworkName,
					IpAddress: "10.1.1.1",
				},
			},
			wantAdditionalNodeNetworks: networkv1.MultiNetworkAnnotation{
				{
					Name:  redNetworkName,
					Scope: "host-local",
					Cidrs: []string{"172.11.1.0/24"},
				},
			},
		},
		{
			desc: "interfaces without matching k8s networks should be ignored",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName),
				network(redNetworkName, redGKENetworkParamsName),
			},
			gkeNwParams: []*networkv1alpha1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			interfaces: []*compute.NetworkInterface{
				interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
				}),
				interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
					{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
				}),
				interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
					{IpCidrRange: "20.28.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
				}),
			},
			wantDefaultNwPodCIDRs: []string{"10.11.1.0/24"},
			wantNorthInterfaces: networkv1.NorthInterfacesAnnotation{
				{
					Network:   redNetworkName,
					IpAddress: "10.1.1.1",
				},
			},
			wantAdditionalNodeNetworks: networkv1.MultiNetworkAnnotation{
				{
					Name:  redNetworkName,
					Scope: "host-local",
					Cidrs: []string{"172.11.1.0/24"},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// setup
			clientSet := fake.NewSimpleClientset()
			nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 1*time.Second).Networking()
			nwInformer := nwInfFactory.V1().Networks()
			gnpInformer := nwInfFactory.V1alpha1().GKENetworkParamSets()
			for _, nw := range tc.networks {
				err := nwInformer.Informer().GetStore().Add(nw)
				if err != nil {
					t.Fatalf("error in test setup, could not create network %s: %v", nw.Name, err)
				}
			}
			for _, gnp := range tc.gkeNwParams {
				err := gnpInformer.Informer().GetStore().Add(gnp)
				if err != nil {
					t.Fatalf("error in test setup, could not create gke network param set %s: %v", gnp.Name, err)
				}
			}
			ca := &cloudCIDRAllocator{
				networksLister: nwInformer.Lister(),
				gnpLister:      gnpInformer.Lister(),
			}
			// test
			gotDefaultNwCIDRs, gotNorthInterfaces, gotAdditionalNodeNetworks, err := ca.PerformMultiNetworkCIDRAllocation(node, tc.interfaces)
			if tc.expectErr && err == nil {
				t.Fatalf("expected error")
			} else if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			assert.Equal(t, tc.wantDefaultNwPodCIDRs, gotDefaultNwCIDRs)
			assert.Equal(t, tc.wantNorthInterfaces, gotNorthInterfaces)
			assert.Equal(t, tc.wantAdditionalNodeNetworks, gotAdditionalNodeNetworks)
		})
	}
}
