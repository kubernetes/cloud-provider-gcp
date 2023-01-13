package gkenetworkparamset

import (
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/onsi/gomega"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/crd/apis/network/v1alpha1"
	"k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/fake"
	gkenetworkparamset "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1alpha1"
	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/component-base/metrics/prometheus/controllers"

	"k8s.io/kubernetes/pkg/controller"
)

type testGKENetworkParamSetController struct {
	ctx           context.Context
	stop          context.CancelFunc
	networkClient *fake.Clientset
	gnpInformer   cache.SharedIndexInformer
	clusterValues gce.TestClusterValues
	controller    *Controller
	metrics       *controllers.ControllerManagerMetrics
	cloud         *gce.Cloud
}

func setupGKENetworkParamSetController() *testGKENetworkParamSetController {
	fakeNetworking := fake.NewSimpleClientset()
	gkeNetworkParamSetInformer := gkenetworkparamset.NewGKENetworkParamSetInformer(fakeNetworking, controller.NoResyncPeriodFunc(), cache.Indexers{})
	testClusterValues := gce.DefaultTestClusterValues()
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)
	controller := NewGKENetworkParamSetController(
		fakeNetworking,
		gkeNetworkParamSetInformer,
		fakeGCE,
	)
	metrics := controllers.NewControllerManagerMetrics("test")

	return &testGKENetworkParamSetController{
		networkClient: fakeNetworking,
		gnpInformer:   gkeNetworkParamSetInformer,
		clusterValues: testClusterValues,
		controller:    controller,
		metrics:       metrics,
		cloud:         fakeGCE,
	}
}

func (testVals *testGKENetworkParamSetController) runGKENetworkParamSetController(ctx context.Context) {
	go testVals.gnpInformer.Run(ctx.Done())
	go testVals.controller.Run(1, ctx.Done(), testVals.metrics)
}

func TestControllerRuns(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController()
	testVals.runGKENetworkParamSetController(ctx)
}

func TestAddValidParamSetSingleSecondaryRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController()

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
	paramSet := &v1alpha1.GKENetworkParamSet{
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: v1alpha1.GKENetworkParamSetSpec{
			VPC:       "default",
			VPCSubnet: subnetName,
			PodIPv4Ranges: &v1alpha1.SecondaryRanges{
				RangeNames: []string{
					subnetSecondaryRangeName,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController()

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
	paramSet := &v1alpha1.GKENetworkParamSet{
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: v1alpha1.GKENetworkParamSetSpec{
			VPC:       "default",
			VPCSubnet: subnetName,
			PodIPv4Ranges: &v1alpha1.SecondaryRanges{
				RangeNames: []string{
					subnetSecondaryRangeName1,
					subnetSecondaryRangeName2,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController()

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
	paramSet := &v1alpha1.GKENetworkParamSet{
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: v1alpha1.GKENetworkParamSetSpec{
			VPC:       "default",
			VPCSubnet: subnetName,
			PodIPv4Ranges: &v1alpha1.SecondaryRanges{
				RangeNames: []string{
					nonExistentSecondaryRangeName,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Consistently(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController()

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
	nonExistentSecondaryRangeName := "test-secondary-does-not-exist"
	paramSet := &v1alpha1.GKENetworkParamSet{
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: v1alpha1.GKENetworkParamSetSpec{
			VPC:       "default",
			VPCSubnet: subnetName,
			PodIPv4Ranges: &v1alpha1.SecondaryRanges{
				RangeNames: []string{
					subnetSecondaryRangeName1,
					nonExistentSecondaryRangeName,
				},
			},
		},
	}
	_, err = testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController()

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
	paramSet := &v1alpha1.GKENetworkParamSet{
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: v1alpha1.GKENetworkParamSetSpec{
			VPC:       "default",
			VPCSubnet: subnetName,
		},
	}
	_, err = testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1alpha1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
