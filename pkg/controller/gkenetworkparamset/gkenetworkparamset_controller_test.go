package gkenetworkparamset

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"slices"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	networkfake "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	condmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/component-base/metrics/prometheus/controllers"
)

type testGKENetworkParamSetController struct {
	networkClient       *networkfake.Clientset
	informerFactory     networkinformers.SharedInformerFactory
	clusterValues       gce.TestClusterValues
	controller          *Controller
	metrics             *controllers.ControllerManagerMetrics
	cloud               *gce.Cloud
	nodeStore           cache.Store
	nodeClient          *fake.Clientset
	nodeInformerFactory informers.SharedInformerFactory
}

const (
	defaultTestNetworkName    = "default-network"
	nonDefaultTestNetworkName = "not-default-network"
	defaultTestSubnetworkName = "default-subnetwork"
	defaultNode               = "default-node"
	node1                     = "new-node1"
	defaultPodRange           = "default-pod-range"
	newPodRange1              = "new-pod-range1"
	newPodRange2              = "new-pod-range2"
	defaultPodCIDR            = "10.100.0.0/16"
	defaultPodIPv6CIDR        = "2600:1900:4000:fd1::/64"
	newPodCIDR1               = "10.101.0.0/16"
	newPodCIDR2               = "10.102.0.0/16"
)

func setupGKENetworkParamSetController(ctx context.Context, clusterCIDR string) *testGKENetworkParamSetController {
	return setupGKENetworkParamSetControllerWithCustomDefaultGNPAndNodeClient(ctx, clusterCIDR, "", fake.NewSimpleClientset())
}

func setupGKENetworkParamSetControllerWithCustomDefaultGNP(ctx context.Context, clusterCIDR string, customDefaultGNPName string) *testGKENetworkParamSetController {
	return setupGKENetworkParamSetControllerWithCustomDefaultGNPAndNodeClient(ctx, clusterCIDR, customDefaultGNPName, fake.NewSimpleClientset())
}

func setupGKENetworkParamSetControllerWithCustomDefaultGNPAndNodeClient(ctx context.Context, clusterCIDR string, customDefaultGNPName string, nodeClient *fake.Clientset) *testGKENetworkParamSetController {
	fakeNetworking := networkfake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)
	nwInformer := nwInfFactory.Networking().V1().Networks()
	gnpInformer := nwInfFactory.Networking().V1().GKENetworkParamSets()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.NetworkURL = fmt.Sprintf("projects/%v/global/networks/%v", testClusterValues.ProjectID, defaultTestNetworkName)
	testClusterValues.SubnetworkURL = fmt.Sprintf("projects/%v/regions/%v/subnetworks/%v", testClusterValues.ProjectID, testClusterValues.Region, defaultTestSubnetworkName)
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	fakeInformerFactory := informers.NewSharedInformerFactory(nodeClient, 0*time.Second)
	fakeNodeInformer := fakeInformerFactory.Core().V1().Nodes()

	_, ipnet, _ := net.ParseCIDR(clusterCIDR)

	controller := NewGKENetworkParamSetController(
		fakeNodeInformer,
		fakeNetworking,
		gnpInformer,
		nwInformer,
		fakeGCE,
		nwInfFactory,
		[]*net.IPNet{ipnet},
		customDefaultGNPName,
	)
	controller.nodeInformerSynced = func() bool { return true }

	metrics := controllers.NewControllerManagerMetrics("test")

	defaultNetworkKey := meta.GlobalKey(defaultTestNetworkName)
	defaultNetwork := &compute.Network{
		Name: defaultTestNetworkName,
	}
	// this should never error as we are not actually making a network call
	fakeGCE.Compute().Networks().Insert(ctx, defaultNetworkKey, defaultNetwork)

	nonDefaultNetworkKey := meta.GlobalKey(nonDefaultTestNetworkName)
	nonDefaultNetwork := &compute.Network{
		Name: nonDefaultTestNetworkName,
	}
	fakeGCE.Compute().Networks().Insert(ctx, nonDefaultNetworkKey, nonDefaultNetwork)

	return &testGKENetworkParamSetController{
		networkClient:       fakeNetworking,
		informerFactory:     nwInfFactory,
		clusterValues:       testClusterValues,
		controller:          controller,
		metrics:             metrics,
		cloud:               fakeGCE,
		nodeStore:           fakeNodeInformer.Informer().GetStore(),
		nodeClient:          nodeClient,
		nodeInformerFactory: fakeInformerFactory,
	}
}

func (testVals *testGKENetworkParamSetController) runGKENetworkParamSetController(ctx context.Context) {
	go testVals.controller.Run(1, ctx.Done(), testVals.metrics)
}

func TestControllerRuns(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)
	testVals.runGKENetworkParamSetController(ctx)
}

func TestControllerRunsIPv6Only(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodIPv6CIDR)
	testVals.runGKENetworkParamSetController(ctx)
}

func TestAddValidParamSetSingleSecondaryRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	subnetName := "test-subnet"
	subnetSecondaryRangeName := "test-secondary-range"
	subnetSecondaryCidr := "10.0.0.0/24"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)
	subnet := &compute.Subnetwork{
		Name: subnetName,
		SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
			{
				IpCidrRange: subnetSecondaryCidr,
				RangeName:   subnetSecondaryRangeName,
			},
		},
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}

	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       defaultTestNetworkName,
			VPCSubnet: subnetName,
			PodIPv4Ranges: &networkv1.SecondaryRanges{
				RangeNames: []string{
					subnetSecondaryRangeName,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetSecondaryCidr))
			return true, nil
		}

		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated with secondary range cidr.")

}

func TestAddValidParamSetMultipleSecondaryRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	subnetName := "test-subnet"
	subnetSecondaryRangeName1 := "test-secondary-range-1"
	subnetSecondaryCidr1 := "10.0.0.1/24"
	subnetSecondaryRangeName2 := "test-secondary-range-2"
	subnetSecondaryCidr2 := "10.0.0.2/24"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)
	subnet := &compute.Subnetwork{
		Name: subnetName,
		SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
			{
				IpCidrRange: subnetSecondaryCidr1,
				RangeName:   subnetSecondaryRangeName1,
			},
			{
				IpCidrRange: subnetSecondaryCidr2,
				RangeName:   subnetSecondaryRangeName2,
			},
		},
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}

	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       defaultTestNetworkName,
			VPCSubnet: subnetName,
			PodIPv4Ranges: &networkv1.SecondaryRanges{
				RangeNames: []string{
					subnetSecondaryRangeName1,
					subnetSecondaryRangeName2,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetSecondaryCidr1, subnetSecondaryCidr2))
			return true, nil
		}

		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated with secondary range cidrs.")

}

func TestAddInvalidParamSetNoMatchingSecondaryRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	subnetName := "test-subnet"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)
	subnet := &compute.Subnetwork{
		Name: subnetName,
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}

	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	nonExistentSecondaryRangeName := "test-secondary-does-not-exist"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       defaultTestNetworkName,
			VPCSubnet: subnetName,
			PodIPv4Ranges: &networkv1.SecondaryRanges{
				RangeNames: []string{
					nonExistentSecondaryRangeName,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Consistently(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			return false, nil
		}

		return true, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should contain an empty list of cidrs")

}

func TestParamSetPartialSecondaryRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	subnetName := "test-subnet"
	subnetSecondaryRangeName1 := "test-secondary-range-1"
	subnetSecondaryCidr1 := "10.0.0.1/24"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)
	subnet := &compute.Subnetwork{
		Name: subnetName,
		SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
			{
				IpCidrRange: subnetSecondaryCidr1,
				RangeName:   subnetSecondaryRangeName1,
			},
		},
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}

	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       defaultTestNetworkName,
			VPCSubnet: subnetName,
			PodIPv4Ranges: &networkv1.SecondaryRanges{
				RangeNames: []string{
					subnetSecondaryRangeName1,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetSecondaryCidr1))
			return true, nil
		}

		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated with secondary range cidr.")

}

func TestValidParamSetSubnetRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	subnetName := "test-subnet"
	subnetCidr := "10.0.0.0/24"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)
	subnet := &compute.Subnetwork{
		Name:        subnetName,
		IpCidrRange: subnetCidr,
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}
	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:        nonDefaultTestNetworkName,
			VPCSubnet:  subnetName,
			DeviceMode: networkv1.NetDevice,
		},
	}
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetCidr))
			return true, nil
		}

		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated with subnet cidr.")

}

func TestValidParamSetSubnetInternalIpv6Prefix(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodIPv6CIDR)

	subnetName := "ipv6-subnet"
	subnetIpv6Cidr := "2001:db8::/32"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)

	subnet := &compute.Subnetwork{
		Name:               subnetName,
		InternalIpv6Prefix: subnetIpv6Cidr,
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}
	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset-ipv6"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:        nonDefaultTestNetworkName,
			VPCSubnet:  subnetName,
			DeviceMode: networkv1.NetDevice,
		},
	}

	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetIpv6Cidr))
			return true, nil
		}

		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated ONLY with subnet internal ipv6 prefix.")
}

func TestValidParamSetSubnetExternalIpv6Prefix(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodIPv6CIDR)
	subnetName := "ipv6-subnet"
	subnetIpv6Cidr := "2001:db8::/32"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)

	subnet := &compute.Subnetwork{
		Name:               subnetName,
		ExternalIpv6Prefix: subnetIpv6Cidr,
	}
	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}
	testVals.runGKENetworkParamSetController(ctx)
	gkeNetworkParamSetName := "test-paramset-ipv6"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: gkeNetworkParamSetName},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:        nonDefaultTestNetworkName,
			VPCSubnet:  subnetName,
			DeviceMode: networkv1.NetDevice,
		},
	}

	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}
	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetIpv6Cidr))
			return true, nil
		}
		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated with subnet cidr.")
}

func TestValidParamSetSubnetIpv6CidrRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodIPv6CIDR)

	subnetName := "ipv6-subnet"
	subnetIpv6Cidr := "2001:db8::/32"
	subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)

	subnet := &compute.Subnetwork{
		Name:          subnetName,
		Ipv6CidrRange: subnetIpv6Cidr,
	}

	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}
	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset-ipv6"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: gkeNetworkParamSetName},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:        nonDefaultTestNetworkName,
			VPCSubnet:  subnetName,
			DeviceMode: networkv1.NetDevice,
		},
	}

	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		cidrExists := paramSet.Status.PodCIDRs != nil && len(paramSet.Status.PodCIDRs.CIDRBlocks) > 0
		if cidrExists {
			g.Ω(paramSet.Status.PodCIDRs.CIDRBlocks).Should(gomega.ConsistOf(subnetIpv6Cidr))
			return true, nil
		}
		return false, nil
	}).Should(gomega.BeTrue(), "GKENetworkParamSet Status should be updated with subnet cidr.")
}

func TestAddAndRemoveFinalizerToGKENetworkParamSet_NoNetworkName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       defaultTestNetworkName,
			VPCSubnet: "test-subnet",
		},
	}
	_, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
	}).Should(gomega.BeTrue(), "GKENetworkParamSet should have the finalizer added.")

	paramSet, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}

	now := metav1.Now()
	paramSet.SetDeletionTimestamp(&now)
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Update(ctx, paramSet, metav1.UpdateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
	}).Should(gomega.BeFalse(), "finalizer should have been removed from GKENetworkParamSet")
}

type conditionMatcher struct {
	expected metav1.Condition
}

func (m *conditionMatcher) Match(actual interface{}) (success bool, err error) {
	actualCondition, ok := actual.(metav1.Condition)
	if !ok {
		return false, fmt.Errorf("expected a metav1.Condition, got %T", actual)
	}

	return actualCondition.Type == m.expected.Type &&
		actualCondition.Status == m.expected.Status &&
		actualCondition.Reason == m.expected.Reason, nil
}

func (m *conditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto match\n\t%#v\nignoring Message and LastTransitionTime fields", actual, m.expected)
}

func (m *conditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to match\n\t%#v\nignoring Message and LastTransitionTime fields", actual, m.expected)
}

func matchConditionIgnoringMessageAndLastTransitionTime(expected metav1.Condition) types.GomegaMatcher {
	return &conditionMatcher{expected: expected}
}

func TestGKENetworkParamSetValidations(t *testing.T) {
	gkeNetworkParamSetName := "test-paramset"
	duplicateVPCName := "already-used-vpc"
	duplicateSubnetName := "already-used-subnet"
	netAttachmentName := "projects/test-project/regions/test-region/networkAttachments/testAttachment"

	tests := []struct {
		name              string
		paramSet          *networkv1.GKENetworkParamSet
		subnet            *compute.Subnetwork
		isIPv6OnlyCluster bool
		expectedCondition metav1.Condition
	}{
		{
			name: "VPC with unspecified VPCSubnet",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC: "test-vpc",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "Unspecified - no NetworkAttachment, no VPC or VPCSubnet",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "Specified Subnet not found",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        "test-vpc",
					VPCSubnet:  "non-existent-test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "SubnetNotFound",
			},
		},
		{
			name: "Secondary range not found",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       nonDefaultTestNetworkName,
					VPCSubnet: "test-subnet",
					PodIPv4Ranges: &networkv1.SecondaryRanges{
						RangeNames: []string{
							"nonexistent-secondary-range",
						},
					},
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "SecondaryRangeNotFound",
			},
		},
		{
			name: "DeviceMode and secondary range specified at the same time",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       nonDefaultTestNetworkName,
					VPCSubnet: "test-subnet",
					PodIPv4Ranges: &networkv1.SecondaryRanges{
						RangeNames: []string{
							"test-secondary-range",
						},
					},
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "DeviceModeCantBeUsedWithSecondaryRange",
			},
		},
		{
			name: "Valid GKENetworkParamSet with VPC, VPCSubnet, and PodIPv4Ranges",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       defaultTestNetworkName,
					VPCSubnet: "test-subnet",
					PodIPv4Ranges: &networkv1.SecondaryRanges{
						RangeNames: []string{
							"test-secondary-range",
						},
					},
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "GNPReady",
			},
		},
		{
			name: "Valid GKENetworkParamSet with NetworkAttachment",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "GNPReady",
			},
		},
		{
			name: "GNP with NetworkAttachment - malformed identifier (unanchored bypass attempt)",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: "malicious-prefix/projects/test-project/regions/test-region/networkAttachments/testAttachment/malicious-suffix",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "NetworkAttachmentInvalid",
			},
		},
		{
			name: "GNP with NetworkAttachment - bad format",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: "invalid",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "NetworkAttachmentInvalid",
			},
		},
		{
			name: "GNP with deviceMode and referencing VPC is referenced in any other existing GNP",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpc-already-in-use-gnp",
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        duplicateVPCName,
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "GNPReady",
			},
		},
		{
			name: "GNP with deviceMode and referencing subnet is referenced in any other existing GNP",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sunet-already-in-use-gnp",
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  duplicateSubnetName,
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "DeviceModeSubnetAlreadyInUse",
			},
		},
		{
			name: "GNP with VPCSubnet - VPC unspecified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "GNP with VPC and VPCSubnet - specified, but nonexistant VPC",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        "VPC-DOES-NOT-EXIST",
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "VPCNotFound",
			},
		},
		{
			name: "GNP with VPC and VPCSubnet - without DeviceMode or secondary range specified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       nonDefaultTestNetworkName,
					VPCSubnet: "test-subnet",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "SecondaryRangeAndDeviceModeUnspecified",
			},
		},
		{
			name: "GNP with deviceMode and the referencing VPC is the default VPC",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        defaultTestNetworkName,
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "DeviceModeCantUseDefaultVPC",
			},
		},
		{
			name: "GNP with NetworkAttachment - but VPC specified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
					VPC:               defaultTestNetworkName,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "GNP with NetworkAttachment - but VPCSubnet specified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
					VPCSubnet:         "test-subnet",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "GNP with NetworkAttachment - but PodIPv4Ranges specified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{
						RangeNames: []string{
							"nonexistent-secondary-range",
						},
					},
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "GNP with NetworkAttachment - but DeviceMode specified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
					DeviceMode:        "test-device-mode",
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "GNPConfigInvalid",
			},
		},
		{
			name: "IPv4/Dual-stack cluster rejects unset GNP.Spec.PodIPv4Ranges for Default Network",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkv1.DefaultPodNetworkName,
					Labels: map[string]string{
						"addonmanager.kubernetes.io/mode": "Reconcile",
					},
					Annotations: map[string]string{},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       "test-vpc",
					VPCSubnet: "test-subnet",
				},
			},
			isIPv6OnlyCluster: false,
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "SecondaryRangeAndDeviceModeUnspecified",
			},
		},
		{
			name: "IPv6-only cluster allows unset GNP.Spec.PodIPv4Ranges for Default Network",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        networkv1.DefaultPodNetworkName,
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       defaultTestNetworkName,
					VPCSubnet: "test-subnet",
				},
			},
			isIPv6OnlyCluster: true,
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "GNPReady",
			},
		},
		{
			name: "IPv6-only cluster rejects unset GNP.Spec.PodIPv4Ranges for custom Network",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "custom-network",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       defaultTestNetworkName,
					VPCSubnet: "test-subnet",
				},
			},
			isIPv6OnlyCluster: true,
			expectedCondition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "SecondaryRangeAndDeviceModeUnspecified",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()

			cidrForEnv := defaultPodCIDR
			if test.isIPv6OnlyCluster {
				cidrForEnv = defaultPodIPv6CIDR
			}
			testVals := setupGKENetworkParamSetController(ctx, cidrForEnv)

			// Create subnet
			subnet := &compute.Subnetwork{
				Name: "test-subnet",
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: "10.0.0.0/24",
						RangeName:   "test-secondary-range",
					},
				},
			}
			subnetKey := meta.RegionalKey(subnet.Name, testVals.clusterValues.Region)
			err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
			if err != nil {
				t.Error(err)
			}

			defaultSubnet := &compute.Subnetwork{
				Name: defaultTestSubnetworkName,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: "10.100.0.0/16",
						RangeName:   "default-pod-range",
					},
				},
			}
			defaultSubnetKey := meta.RegionalKey(defaultSubnet.Name, testVals.clusterValues.Region)
			err = testVals.cloud.Compute().Subnetworks().Insert(ctx, defaultSubnetKey, defaultSubnet)
			if err != nil {
				t.Error(err)
			}

			testVals.runGKENetworkParamSetController(ctx)

			// Create network resources used by duplicate gnps
			duplicateNetworkKey := meta.GlobalKey(duplicateVPCName)
			duplicateNetwork := &compute.Network{
				Name: duplicateVPCName,
			}
			err = testVals.cloud.Compute().Networks().Insert(ctx, duplicateNetworkKey, duplicateNetwork)
			if err != nil {
				t.Error(err)
			}
			duplicateSubnet := &compute.Subnetwork{
				Name: duplicateSubnetName,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: "10.0.0.0/24",
						RangeName:   "test-secondary-range",
					},
				},
			}
			duplicateSubnetKey := meta.RegionalKey(duplicateSubnetName, testVals.clusterValues.Region)
			err = testVals.cloud.Compute().Subnetworks().Insert(ctx, duplicateSubnetKey, duplicateSubnet)
			if err != nil {
				t.Error(err)
			}
			oldGNP := &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-paramset",
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       duplicateVPCName,
					VPCSubnet: duplicateSubnetName,
				},
			}
			now := time.Now()
			afterNow := now.Add(1 * time.Minute)
			oldGNP.CreationTimestamp = metav1.NewTime(now)
			test.paramSet.CreationTimestamp = metav1.NewTime(afterNow)

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, oldGNP, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create existing GNP: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.paramSet, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create GKENetworkParamSet: %v", err)
			}

			// Wait for the conditions to be updated by the controller
			g.Eventually(func() (metav1.Condition, error) {
				updatedParamSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, test.paramSet.Name, metav1.GetOptions{})
				if err != nil {
					return metav1.Condition{}, err
				}

				for _, condition := range updatedParamSet.Status.Conditions {
					if condition.Type == "Ready" {
						return condition, nil
					}
				}

				return metav1.Condition{}, fmt.Errorf("GKENetworkParamSet Ready condition not found")
			}).Should(matchConditionIgnoringMessageAndLastTransitionTime(test.expectedCondition), "GKENetworkParamSet condition should match the expected condition")

		})
	}
}

func TestCrossValidateNetworkAndGnp(t *testing.T) {
	gkeNetworkParamSetName := "test-paramset"
	subnetName := "test-subnet"
	subnetSecondaryRangeName := "test-secondary-range"
	networkName := "network-name"
	netAttachmentName := "projects/test-project/regions/test-region/networkAttachments/testAttachment"

	tests := []struct {
		name              string
		network           *networkv1.Network
		paramSet          *networkv1.GKENetworkParamSet
		subnetStackType   string
		isIPv6OnlyCluster bool
		expectedCondition metav1.Condition
	}{
		{
			name: "L3NetworkType has VPC + VPCSubnet GNP missing PodIPv4Ranges",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "L3SecondaryMissing",
			},
		},
		{
			name: "L3NetworkType with IPV6_ONLY subnet",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			subnetStackType: "IPV6_ONLY",
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "SubnetStackTypeIncompatible",
			},
		},
		{
			name: "L3NetworkType Default network with IPV6_ONLY subnet on IPv6Only Cluster",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{Name: networkv1.DefaultPodNetworkName},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: networkv1.DefaultPodNetworkName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        networkv1.DefaultPodNetworkName,
					Annotations: map[string]string{},
					Labels: map[string]string{
						"addonmanager.kubernetes.io/mode": "Reconcile",
					},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			subnetStackType:   "IPV6_ONLY",
			isIPv6OnlyCluster: true,
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "L3NetworkType Default network with IPV6_ONLY subnet on DualStack/IPv4 Cluster",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{Name: networkv1.DefaultPodNetworkName},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: networkv1.DefaultPodNetworkName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        networkv1.DefaultPodNetworkName,
					Annotations: map[string]string{},
					Labels: map[string]string{
						"addonmanager.kubernetes.io/mode": "Reconcile",
					},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			subnetStackType:   "IPV6_ONLY",
			isIPv6OnlyCluster: false,
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "SubnetStackTypeIncompatible",
			},
		},
		{
			name: "DeviceNetworkType with NetworkAttachment",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "NetworkAttachmentUnsupported",
			},
		},
		{
			name: "L2NetworkType with NetworkAttachment",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L2NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "NetworkAttachmentUnsupported",
			},
		},
		{
			name: "DeviceNetworkType with missing DeviceMode",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "DeviceModeMissing",
			},
		},
		{
			name: "Valid L3NetworkType with PodIPv4Ranges",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "Valid L3NetworkType with NetworkAttachment",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					NetworkAttachment: netAttachmentName,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "Valid DeviceNetworkType",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "Valid and Network has mixed case kind in ParametersRef",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: "GkEnetworkparamSet"},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "IPv6-only cluster allows L3NetworkType with unset GNP.Spec.PodIPv4Ranges for Default Network",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkv1.DefaultPodNetworkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: networkv1.DefaultPodNetworkName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkv1.DefaultPodNetworkName,
					Labels: map[string]string{
						"addonmanager.kubernetes.io/mode": "Reconcile",
					},
					Annotations: map[string]string{},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       nonDefaultTestNetworkName,
					VPCSubnet: subnetName,
				},
			},
			isIPv6OnlyCluster: true,
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "IPv6-only cluster rejects L3NetworkType with unset GNP.Spec.PodIPv4Ranges for custom Network",
			network: &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{Name: networkName},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: gkeNetworkParamSetName,
					Labels: map[string]string{
						"addonmanager.kubernetes.io/mode": "Reconcile",
					},
					Annotations: map[string]string{},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			isIPv6OnlyCluster: true,
			expectedCondition: metav1.Condition{
				Type:   "ParamsReady",
				Status: metav1.ConditionFalse,
				Reason: "L3SecondaryMissing",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()

			cidrForEnv := defaultPodCIDR
			if test.isIPv6OnlyCluster {
				cidrForEnv = defaultPodIPv6CIDR
			}
			testVals := setupGKENetworkParamSetController(ctx, cidrForEnv)

			subnetSecondaryCidr := "10.0.0.1/24"
			subnetKey := meta.RegionalKey(subnetName, testVals.clusterValues.Region)
			subnet := &compute.Subnetwork{
				Name:      subnetName,
				StackType: test.subnetStackType,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: subnetSecondaryCidr,
						RangeName:   subnetSecondaryRangeName,
					},
				},
			}
			err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
			if err != nil {
				t.Error(err)
			}

			defaultSubnet := &compute.Subnetwork{
				Name: defaultTestSubnetworkName,
			}
			defaultSubnetKey := meta.RegionalKey(defaultSubnet.Name, testVals.clusterValues.Region)
			err = testVals.cloud.Compute().Subnetworks().Insert(ctx, defaultSubnetKey, defaultSubnet)
			if err != nil {
				t.Error(err)
			}

			testVals.runGKENetworkParamSetController(ctx)

			_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, test.network, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create Network: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.paramSet, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create GKENetworkParamSet: %v", err)
			}

			// Wait for the conditions to be updated by the controller
			g.Eventually(func() (metav1.Condition, error) {
				updatedNetwork, err := testVals.networkClient.NetworkingV1().Networks().Get(ctx, test.network.Name, metav1.GetOptions{})
				if err != nil {
					return metav1.Condition{}, err
				}
				for _, condition := range updatedNetwork.Status.Conditions {
					if condition.Type == "ParamsReady" {
						return condition, nil
					}
				}

				return metav1.Condition{}, fmt.Errorf("Network ParamsReady condition not found")
			}).Should(matchConditionIgnoringMessageAndLastTransitionTime(test.expectedCondition), "Network ParamsReady condition should match the expected condition")
		})
	}

}

func TestHandleGKENetworkParamSetDelete_NetworkPresent(t *testing.T) {

	tests := []struct {
		name            string
		networkUpdateFn func(ctx context.Context, networkName string, networkClient *networkfake.Clientset)
	}{
		{name: "Network no longer refer to GNP",
			networkUpdateFn: func(ctx context.Context, networkName string, networkClient *networkfake.Clientset) {
				network, err := networkClient.NetworkingV1().Networks().Get(ctx, networkName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get Network: %v", err)
				}
				network.Spec.ParametersRef.Name = "non-existent"
				_, err = networkClient.NetworkingV1().Networks().Update(ctx, network, metav1.UpdateOptions{})
				if err != nil {
					t.Fatalf("Failed to update Network status: %v", err)
				}
			},
		},
		{name: "Network deleted",
			networkUpdateFn: func(ctx context.Context, networkName string, networkClient *networkfake.Clientset) {
				err := networkClient.NetworkingV1().Networks().Delete(ctx, networkName, metav1.DeleteOptions{})
				if err != nil {
					t.Fatalf("Failed to update Network: %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()

			testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

			subnetName := "test-subnet"
			subnet := &compute.Subnetwork{
				Name: subnetName,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: "10.0.0.0/24",
						RangeName:   "test-secondary-range",
					},
				},
			}
			subnetKey := meta.RegionalKey(subnet.Name, testVals.clusterValues.Region)
			err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
			if err != nil {
				t.Error(err)
			}

			testVals.runGKENetworkParamSetController(ctx)

			networkName := "test-network"
			gkeNetworkParamSetName := "test-paramset"

			network := &networkv1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			}

			paramSet := &networkv1.GKENetworkParamSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:       gkeNetworkParamSetName,
					Finalizers: []string{GNPFinalizer},
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
				Status: networkv1.GKENetworkParamSetStatus{
					NetworkName: networkName,
				},
			}

			_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, network, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create Network: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create GKENetworkParamSet: %v", err)
			}

			g.Consistently(func() (bool, error) {
				return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
			}).Should(gomega.BeTrue(), "finalizer should exist on GKENetworkParamSet")

			g.Eventually(func() bool {
				network, err := testVals.networkClient.NetworkingV1().Networks().Get(ctx, networkName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get Network: %v", err)
				}

				return condmeta.IsStatusConditionTrue(network.Status.Conditions, "ParamsReady")
			}).Should(gomega.BeTrue(), "ParamsReady should be true in Network Conditions")

			newParamset, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get GKENetworkParamSet: %v", err)
			}

			// simulate a delete on GNP resource
			now := metav1.Now()
			newParamset.SetDeletionTimestamp(&now)
			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Update(ctx, newParamset, metav1.UpdateOptions{})
			if err != nil {
				t.Fatalf("Failed to update GKENetworkParamSet: %v", err)
			}

			test.networkUpdateFn(ctx, networkName, testVals.networkClient)

			g.Eventually(func() (bool, error) {
				return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
			}).Should(gomega.BeFalse(), "finalizer should be removed from GKENetworkParamSet")

			// The finalizer was removed, we need to manually handle the deletion
			err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Delete(ctx, gkeNetworkParamSetName, metav1.DeleteOptions{})
			if err != nil {
				t.Fatalf("Failed to delete GKENetworkParamSet: %v", err)
			}

			// networkUpdateFn can delete network, so we only want to make an assertion
			// on network conditions if it still exists
			networkWasDeleted := false
			_, err = testVals.networkClient.NetworkingV1().Networks().Get(ctx, networkName, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				networkWasDeleted = true
			} else if err != nil {
				t.Fatalf("Failed to get Network: %v", err)
			}

			if !networkWasDeleted {
				g.Eventually(func() bool {
					network, err := testVals.networkClient.NetworkingV1().Networks().Get(ctx, networkName, metav1.GetOptions{})
					if err != nil {
						t.Fatalf("Failed to get Network: %v", err)
					}
					return condmeta.IsStatusConditionFalse(network.Status.Conditions, "ParamsReady")
				}).Should(gomega.BeTrue(), "ParamsReady should be removed from Network Conditions")
			}

		})
	}
}

func (testVals *testGKENetworkParamSetController) doesGNPFinalizerExist(ctx context.Context, gkeNetworkParamSetName string) (bool, error) {
	paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, finalizer := range paramSet.ObjectMeta.Finalizers {
		if finalizer == GNPFinalizer {
			return true, nil
		}
	}

	return false, nil
}

func TestPopulateDesiredDefaultParamSet(t *testing.T) {
	desiredDefaultParamSet := newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, nil)

	ipv6DesiredDefaultParamSet := newL3GNP(networkv1.DefaultPodNetworkName, nil, nil)

	tests := []struct {
		name              string
		defaultParamSet   *networkv1.GKENetworkParamSet
		wantParamSet      *networkv1.GKENetworkParamSet
		isIPv6OnlyCluster bool
	}{
		{
			name:            "not populate if addon is reconcile mode",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, nil, &gnpOptions{testAddonMode: reconcileMode}),
			wantParamSet:    newL3GNP(networkv1.DefaultPodNetworkName, nil, &gnpOptions{testAddonMode: reconcileMode}),
		},
		{
			name:            "revert vpc",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, &gnpOptions{vpc: "random-vpc"}),
			wantParamSet:    desiredDefaultParamSet,
		},
		{
			name:            "revert vpc subnet",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, &gnpOptions{subnet: "random-subnet"}),
			wantParamSet:    desiredDefaultParamSet,
		},
		{
			name:            "revert pod range name",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, nil, nil),
			wantParamSet:    desiredDefaultParamSet,
		},
		{
			name:            "revert annotion",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, &gnpOptions{testComponentName: ""}),
			wantParamSet:    desiredDefaultParamSet,
		},
		{
			name:            "revert label",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, &gnpOptions{testAddonMode: ""}),
			wantParamSet:    desiredDefaultParamSet,
		},
		{
			name:              "ipv6-only cluster ignores and clears PodIPv4Ranges",
			defaultParamSet:   newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, nil),
			wantParamSet:      ipv6DesiredDefaultParamSet,
			isIPv6OnlyCluster: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()

			cidrForEnv := defaultPodCIDR
			if test.isIPv6OnlyCluster {
				cidrForEnv = defaultPodIPv6CIDR
			}
			testVals := setupGKENetworkParamSetController(ctx, cidrForEnv)

			subnetKey := meta.RegionalKey(defaultTestSubnetworkName, testVals.clusterValues.Region)
			subnet := &compute.Subnetwork{
				Name: defaultTestSubnetworkName,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: defaultPodCIDR,
						RangeName:   defaultPodRange,
					},
				},
			}
			err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
			if err != nil {
				t.Error(err)
			}

			testVals.runGKENetworkParamSetController(ctx)

			_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, newL3Network(networkv1.DefaultPodNetworkName), metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create Network: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.defaultParamSet, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create default GKENetworkParamSet: %v", err)
			}

			g.Eventually(func() (bool, error) {
				paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, networkv1.DefaultPodNetworkName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if paramSet.Spec.VPC != test.wantParamSet.Spec.VPC ||
					paramSet.Spec.VPCSubnet != test.wantParamSet.Spec.VPCSubnet ||
					!samePodIPv4Ranges(paramSet, test.wantParamSet) ||
					!reflect.DeepEqual(paramSet.Annotations, test.wantParamSet.Annotations) ||
					!reflect.DeepEqual(paramSet.ObjectMeta.Labels, test.wantParamSet.ObjectMeta.Labels) {
					t.Logf("TestPopulateDesiredDefaultParamSet diff (-want +got): \n%v", cmp.Diff(test.wantParamSet, paramSet))
					return false, fmt.Errorf("Default NetworkParamSet should be set to desired state")
				}
				return true, nil
			}).Should(gomega.BeTrue())
		})
	}
}

func TestSyncDefaultPodRanges(t *testing.T) {
	gkeNetworkParamSetName := "test-paramset"

	defaultNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultNode,
			Labels: map[string]string{
				utilnode.NodePoolPodRangeLabelPrefix: defaultPodRange,
			},
		},
	}
	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: node1,
			Labels: map[string]string{
				utilnode.NodePoolPodRangeLabelPrefix: newPodRange1,
			},
		},
	}
	tests := []struct {
		name               string
		defaultParamSet    *networkv1.GKENetworkParamSet
		nonDefaultParamSet *networkv1.GKENetworkParamSet
		nodeList           []*v1.Node
		expectedRanges     []string
	}{
		{
			name:            "No node",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, nil),
			expectedRanges:  []string{defaultPodRange},
		},
		{
			name:            "No pod range label",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, nil),
			nodeList: []*v1.Node{defaultNode, {
				ObjectMeta: metav1.ObjectMeta{
					Name:   "no-label-node",
					Labels: map[string]string{},
				},
			}},
			expectedRanges: []string{defaultPodRange},
		},
		{
			name:            "Pod range label returns empty",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, nil),
			nodeList: []*v1.Node{defaultNode, {
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-label-value-node",
					Labels: map[string]string{
						utilnode.NodePoolPodRangeLabelPrefix: "",
					},
				},
			}},
			expectedRanges: []string{defaultPodRange},
		},
		{
			name:            "No-opt in Reconcile mode",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange}, &gnpOptions{testAddonMode: reconcileMode}),
			nodeList:        []*v1.Node{node1},
			expectedRanges:  []string{defaultPodRange},
		},
		{
			name:            "Default params udpate PodIPv4Range basing on the nodes",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodRange, newPodRange2}, nil),
			nodeList:        []*v1.Node{node1},
			expectedRanges:  []string{defaultPodRange, newPodRange1},
		},
		{
			name:            "PodIPv4Range order does not matter",
			defaultParamSet: newL3GNP(networkv1.DefaultPodNetworkName, []string{newPodRange1, defaultPodCIDR}, nil),
			nodeList:        []*v1.Node{node1, defaultNode},
			expectedRanges:  []string{defaultPodRange, newPodRange1},
		},
		{
			name:               "Non-default params should not update",
			defaultParamSet:    newL3GNP(networkv1.DefaultPodNetworkName, []string{defaultPodCIDR}, nil),
			nonDefaultParamSet: newL3GNP(gkeNetworkParamSetName, []string{newPodRange2}, &gnpOptions{testAddonMode: reconcileMode}),
			nodeList:           []*v1.Node{defaultNode, node1},
			expectedRanges:     []string{defaultPodRange, newPodRange1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

			subnetKey := meta.RegionalKey(defaultTestSubnetworkName, testVals.clusterValues.Region)
			subnet := &compute.Subnetwork{
				Name: defaultTestSubnetworkName,
				SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
					{
						IpCidrRange: defaultPodCIDR,
						RangeName:   defaultPodRange,
					},
					{
						IpCidrRange: newPodCIDR1,
						RangeName:   newPodRange1,
					},
					{
						IpCidrRange: newPodCIDR2,
						RangeName:   newPodRange2,
					},
				},
			}
			err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
			if err != nil {
				t.Error(err)
			}

			testVals.runGKENetworkParamSetController(ctx)

			_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, newL3Network(networkv1.DefaultPodNetworkName), metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create Network: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.defaultParamSet, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create default GKENetworkParamSet: %v", err)
			}

			if test.nonDefaultParamSet != nil {
				_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.nonDefaultParamSet, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create non-default GKENetworkParamSet: %v", err)
				}
				_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, newL3Network(gkeNetworkParamSetName), metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create non-default Network: %v", err)
				}
			}

			if test.nodeList != nil {
				for _, n := range test.nodeList {
					err = testVals.nodeStore.Add(n)
					if err != nil {
						t.Error(err)
					}
				}
			}

			// Wait for the pod ranges to be updated by the controller
			g.Eventually(func() (bool, error) {
				// no range name change on non-default params
				if test.nonDefaultParamSet != nil {
					paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, test.nonDefaultParamSet.Name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if paramSet.Spec.PodIPv4Ranges != nil && len(paramSet.Spec.PodIPv4Ranges.RangeNames) > 1 ||
						paramSet.Spec.PodIPv4Ranges.RangeNames[0] != test.nonDefaultParamSet.Spec.PodIPv4Ranges.RangeNames[0] {
						return false, fmt.Errorf("Non default params should not update")
					}
				}

				paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, networkv1.DefaultPodNetworkName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if paramSet.Spec.PodIPv4Ranges == nil {
					return test.expectedRanges == nil, nil
				}
				if sameStringSlice(paramSet.Spec.PodIPv4Ranges.RangeNames, test.expectedRanges) {
					return true, nil
				}

				return false, fmt.Errorf("NetworkParamSet has the wrong Pod IPv4 ranges")
			}).Should(gomega.BeTrue(), "Network Params Pod IPv4 ranges should match the expected ranges")
		})
	}
}

type gnpOptions struct {
	vpc                string
	subnet             string
	testAddonMode      string
	testComponentLayer string
	testComponentName  string
}

// newL3GNP returns new L3 paramset object for test
func newL3GNP(name string, rangeNames []string, opts *gnpOptions) *networkv1.GKENetworkParamSet {
	gnp := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				annotationComponentsLayer: componentLayer,
				annotationComponentsName:  componentName,
			},
			Labels: map[string]string{
				labelsAddonManagerMode: ensureExistsMode,
			},
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:           defaultTestNetworkName,
			VPCSubnet:     defaultTestSubnetworkName,
			PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: rangeNames},
		},
	}
	if opts != nil {
		if opts.testAddonMode != "" {
			gnp.ObjectMeta.Labels[labelsAddonManagerMode] = opts.testAddonMode
		}
		if opts.testComponentLayer != "" {
			gnp.ObjectMeta.Annotations[annotationComponentsLayer] = opts.testComponentLayer
		}
		if opts.testComponentName != "" {
			gnp.ObjectMeta.Annotations[annotationComponentsName] = opts.testComponentName
		}
		if opts.vpc != "" {
			gnp.Spec.VPC = opts.vpc
		}
		if opts.subnet != "" {
			gnp.Spec.VPCSubnet = opts.subnet
		}
	}
	return gnp
}

// newL3Network returns Network object for L3 type
func newL3Network(name string) *networkv1.Network {
	network := &networkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1.NetworkSpec{
			Type:          networkv1.L3NetworkType,
			ParametersRef: &networkv1.NetworkParametersReference{Name: name, Kind: gnpKind},
		},
	}
	return network
}

func TestSameStringSlice(t *testing.T) {
	tests := []struct {
		name string
		x    []string
		y    []string
		want bool
	}{
		{
			name: "not same len",
			x:    []string{"a"},
			y:    []string{""},
			want: false,
		},
		{
			name: "same order",
			x:    []string{"ab", "bc"},
			y:    []string{"ab", "bc"},
			want: true,
		},
		{
			name: "not same order",
			x:    []string{"ab", "bc"},
			y:    []string{"bc", "ab"},
			want: true,
		},
		{
			name: "counted each elements",
			x:    []string{"a", "a", "b"},
			y:    []string{"a", "b", "a"},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sameStringSlice(tc.x, tc.y)
			if got != tc.want {
				t.Fatalf("sameStringSlice(%+v) returns %v but want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestSyncDefaultPodRangesWithCustomDefaultGNPName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	customDefaultName := "t365985285473-tenantuno-default"

	testVals := setupGKENetworkParamSetControllerWithCustomDefaultGNP(ctx, defaultPodCIDR, customDefaultName)

	subnetKey := meta.RegionalKey(defaultTestSubnetworkName, testVals.clusterValues.Region)
	subnet := &compute.Subnetwork{
		Name: defaultTestSubnetworkName,
		SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
			{
				IpCidrRange: defaultPodCIDR,
				RangeName:   defaultPodRange,
			},
			{
				IpCidrRange: newPodCIDR1,
				RangeName:   newPodRange1,
			},
		},
	}
	err := testVals.cloud.Compute().Subnetworks().Insert(ctx, subnetKey, subnet)
	if err != nil {
		t.Error(err)
	}

	testVals.runGKENetworkParamSetController(ctx)

	// Create Network and GNP using customDefaultName
	_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, newL3Network(customDefaultName), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Network: %v", err)
	}

	defaultParamSet := newL3GNP(customDefaultName, []string{defaultPodRange}, nil)
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, defaultParamSet, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create default GKENetworkParamSet: %v", err)
	}

	// Add node with new pod range label
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: node1,
			Labels: map[string]string{
				utilnode.NodePoolPodRangeLabelPrefix: newPodRange1,
			},
		},
	}
	err = testVals.nodeStore.Add(node)
	if err != nil {
		t.Error(err)
	}

	// Wait for the custom default GNP's pod ranges to be updated by the controller
	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, customDefaultName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if paramSet.Spec.PodIPv4Ranges == nil {
			return false, fmt.Errorf("PodIPv4Ranges is nil")
		}
		expectedRanges := []string{defaultPodRange, newPodRange1}
		if sameStringSlice(paramSet.Spec.PodIPv4Ranges.RangeNames, expectedRanges) {
			return true, nil
		}
		return false, fmt.Errorf("NetworkParamSet has the wrong Pod IPv4 ranges: expected %+v, got %+v", expectedRanges, paramSet.Spec.PodIPv4Ranges.RangeNames)
	}).Should(gomega.BeTrue(), "Network Params Pod IPv4 ranges for custom default GNP should match the expected ranges")
}

func TestNodeUpdateTriggersCustomDefaultGNPQueue(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	customDefaultName := "t365985285473-tenantuno-default"
	nodeClient := fake.NewSimpleClientset()
	testVals := setupGKENetworkParamSetControllerWithCustomDefaultGNPAndNodeClient(ctx, defaultPodCIDR, customDefaultName, nodeClient)

	// Start both informer factories
	testVals.informerFactory.Start(ctx.Done())
	testVals.nodeInformerFactory.Start(ctx.Done())

	// Create custom default GNP first
	defaultParamSet := newL3GNP(customDefaultName, []string{defaultPodRange}, nil)
	_, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, defaultParamSet, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create default GKENetworkParamSet: %v", err)
	}

	// Wait for GNP informer cache to sync
	testVals.informerFactory.WaitForCacheSync(ctx.Done())

	// Add node with new pod range label (which is different from defaultPodRange, e.g. newPodRange1)
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				utilnode.NodePoolPodRangeLabelPrefix: newPodRange1,
			},
		},
	}

	// Create node in client to trigger events
	_, err = nodeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	// Verify that the customDefaultName gets added to the queue
	gomega.NewGomegaWithT(t).Eventually(func() bool {
		return testVals.controller.queue.Len() > 0
	}, 5*time.Second, 100*time.Millisecond).Should(gomega.BeTrue(), "expected custom default GNP name to be enqueued on node addition")

	item, quit := testVals.controller.queue.Get()
	if quit {
		t.Fatalf("queue quit unexpectedly")
	}
	if item.(string) != customDefaultName {
		t.Errorf("expected enqueued item to be %q, got %q", customDefaultName, item.(string))
	}
}

func TestRemoveTenantParamSetFinalizers(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	tenantName := "t12345-tenantuno"
	otherTenantName := "t99999-tenantdos"

	testVals := setupGKENetworkParamSetController(ctx, defaultPodCIDR)

	// Create GNPs for tenant-1 (one with finalizer, one without)
	gnp1 := newL3GNP("tenant1-default", []string{defaultPodRange}, nil)
	gnp1.Labels = map[string]string{
		ProviderConfigLabelKey: tenantName,
	}
	gnp1.Finalizers = []string{GNPFinalizer}

	gnp2 := newL3GNP("tenant1-custom", []string{newPodRange1}, nil)
	gnp2.Labels = map[string]string{
		ProviderConfigLabelKey: tenantName,
	}

	// Create GNP for tenant-2
	gnpOther := newL3GNP("tenant2-default", []string{defaultPodRange}, nil)
	gnpOther.Labels = map[string]string{
		ProviderConfigLabelKey: otherTenantName,
	}
	gnpOther.Finalizers = []string{GNPFinalizer}

	for _, g := range []*networkv1.GKENetworkParamSet{gnp1, gnp2, gnpOther} {
		_, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, g, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create GNP %s: %v", g.Name, err)
		}
	}

	// 1. Guard check: empty or default supervisor name should return error
	if err := testVals.controller.RemoveTenantParamSetFinalizers(ctx, ""); err == nil {
		t.Errorf("expected error for empty tenant name, got nil")
	}
	if err := testVals.controller.RemoveTenantParamSetFinalizers(ctx, "default"); err == nil {
		t.Errorf("expected error for 'default' tenant name, got nil")
	}

	// 2. Remove finalizers for tenant-1 GNPs
	if err := testVals.controller.RemoveTenantParamSetFinalizers(ctx, tenantName); err != nil {
		t.Fatalf("RemoveTenantParamSetFinalizers failed: %v", err)
	}

	// 3. Verify tenant-1 GNPs still exist but finalizer is removed
	gnp1Check, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, "tenant1-default", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected tenant1-default to exist, got err: %v", err)
	}
	if slices.Contains(gnp1Check.Finalizers, GNPFinalizer) {
		t.Errorf("expected tenant1-default finalizer to be removed, got %v", gnp1Check.Finalizers)
	}

	gnp2Check, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, "tenant1-custom", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected tenant1-custom to exist, got err: %v", err)
	}
	if slices.Contains(gnp2Check.Finalizers, GNPFinalizer) {
		t.Errorf("expected tenant1-custom finalizer to be removed, got %v", gnp2Check.Finalizers)
	}

	// 4. Verify tenant-2 GNP is NOT changed and still has its finalizer
	remainingGNP, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, "tenant2-default", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected tenant-2 GNP to remain, got err: %v", err)
	}
	if remainingGNP == nil {
		t.Errorf("expected tenant-2 GNP to exist")
	}
	if !slices.Contains(remainingGNP.Finalizers, GNPFinalizer) {
		t.Errorf("expected tenant-2 GNP to retain finalizer, got %v", remainingGNP.Finalizers)
	}
}
