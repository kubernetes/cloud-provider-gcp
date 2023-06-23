package gkenetworkparamset

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	condmeta "k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/fake"
	networkinformers "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions"

	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/component-base/metrics/prometheus/controllers"
)

type testGKENetworkParamSetController struct {
	networkClient   *fake.Clientset
	informerFactory networkinformers.SharedInformerFactory
	clusterValues   gce.TestClusterValues
	controller      *Controller
	metrics         *controllers.ControllerManagerMetrics
	cloud           *gce.Cloud
}

const (
	defaultTestNetworkName    = "default-network"
	nonDefaultTestNetworkName = "not-default-network"
)

func setupGKENetworkParamSetController(ctx context.Context) *testGKENetworkParamSetController {
	fakeNetworking := fake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)
	nwInformer := nwInfFactory.Networking().V1().Networks()
	gnpInformer := nwInfFactory.Networking().V1().GKENetworkParamSets()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.NetworkURL = fmt.Sprintf("projects/%v/global/network/%v", testClusterValues.ProjectID, defaultTestNetworkName)
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)
	controller := NewGKENetworkParamSetController(
		fakeNetworking,
		gnpInformer,
		nwInformer,
		fakeGCE,
		nwInfFactory,
	)
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
		networkClient:   fakeNetworking,
		informerFactory: nwInfFactory,
		clusterValues:   testClusterValues,
		controller:      controller,
		metrics:         metrics,
		cloud:           fakeGCE,
	}
}

func (testVals *testGKENetworkParamSetController) runGKENetworkParamSetController(ctx context.Context) {
	go testVals.controller.Run(1, ctx.Done(), testVals.metrics)
}

func TestControllerRuns(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx)
	testVals.runGKENetworkParamSetController(ctx)
}

func TestAddValidParamSetSingleSecondaryRange(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx)

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
		ObjectMeta: v1.ObjectMeta{
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
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController(ctx)

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
		ObjectMeta: v1.ObjectMeta{
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
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController(ctx)

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
		ObjectMeta: v1.ObjectMeta{
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
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Consistently(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController(ctx)

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
		ObjectMeta: v1.ObjectMeta{
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
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
	testVals := setupGKENetworkParamSetController(ctx)

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
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:        nonDefaultTestNetworkName,
			VPCSubnet:  subnetName,
			DeviceMode: networkv1.NetDevice,
		},
	}
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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

func TestAddAndRemoveFinalizerToGKENetworkParamSet_NoNetworkName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	testVals := setupGKENetworkParamSetController(ctx)

	testVals.runGKENetworkParamSetController(ctx)

	gkeNetworkParamSetName := "test-paramset"
	paramSet := &networkv1.GKENetworkParamSet{
		ObjectMeta: v1.ObjectMeta{
			Name: gkeNetworkParamSetName,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       defaultTestNetworkName,
			VPCSubnet: "test-subnet",
		},
	}
	_, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
	}).Should(gomega.BeTrue(), "GKENetworkParamSet should have the finalizer added.")

	paramSet, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
	if err != nil {
		t.Error(err)
	}

	now := v1.Now()
	paramSet.SetDeletionTimestamp(&now)
	_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Update(ctx, paramSet, v1.UpdateOptions{})
	if err != nil {
		t.Error(err)
	}

	g.Eventually(func() (bool, error) {
		return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
	}).Should(gomega.BeFalse(), "finalizer should have been removed from GKENetworkParamSet")
}

type conditionMatcher struct {
	expected v1.Condition
}

func (m *conditionMatcher) Match(actual interface{}) (success bool, err error) {
	actualCondition, ok := actual.(v1.Condition)
	if !ok {
		return false, fmt.Errorf("expected a v1.Condition, got %T", actual)
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

func matchConditionIgnoringMessageAndLastTransitionTime(expected v1.Condition) types.GomegaMatcher {
	return &conditionMatcher{expected: expected}
}

func TestGKENetworkParamSetValidations(t *testing.T) {
	gkeNetworkParamSetName := "test-paramset"
	duplicateVPCName := "already-used-vpc"
	duplicateSubnetName := "already-used-subnet"

	tests := []struct {
		name              string
		paramSet          *networkv1.GKENetworkParamSet
		subnet            *compute.Subnetwork
		expectedCondition v1.Condition
	}{
		{
			name: "Unspecified Subnet",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC: "test-vpc",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "SubnetNotFound",
			},
		},
		{
			name: "Specified Subnet not found",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       "test-vpc",
					VPCSubnet: "non-existant-test-subnet",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "SubnetNotFound",
			},
		},
		{
			name: "Secondary range not found",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
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
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "SecondaryRangeNotFound",
			},
		},
		{
			name: "DeviceMode and secondary range specified at the same time",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
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
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "DeviceModeCantBeUsedWithSecondaryRange",
			},
		},
		{
			name: "Valid GKENetworkParamSet",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
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
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionTrue,
				Reason: "GNPReady",
			},
		},
		{
			name: "GNP with deviceMode and referencing VPC is referenced in any other existing GNP",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: "vpc-already-in-use-gnp",
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        duplicateVPCName,
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "DeviceModeVPCAlreadyInUse",
			},
		},
		{
			name: "GNP with deviceMode and referencing subnet is referenced in any other existing GNP",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: "sunet-already-in-use-gnp",
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  duplicateSubnetName,
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "DeviceModeSubnetAlreadyInUse",
			},
		},
		{
			name: "GNP with VPC unspecified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "VPCNotFound",
			},
		},
		{
			name: "GNP with specified, but nonexistant VPC",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        "VPC-DOES-NOT-EXIST",
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "VPCNotFound",
			},
		},
		{
			name: "GNP without devicemode or secondary range specified",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       nonDefaultTestNetworkName,
					VPCSubnet: "test-subnet",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "SecondaryRangeAndDeviceModeUnspecified",
			},
		},
		{
			name: "GNP with deviceMode and the referencing VPC is the default VPC",
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        defaultTestNetworkName,
					VPCSubnet:  "test-subnet",
					DeviceMode: "test-device-mode",
				},
			},
			expectedCondition: v1.Condition{
				Type:   "Ready",
				Status: v1.ConditionFalse,
				Reason: "DeviceModeCantUseDefaultVPC",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			testVals := setupGKENetworkParamSetController(ctx)

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
				ObjectMeta: v1.ObjectMeta{
					Name: "existing-paramset",
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:       duplicateVPCName,
					VPCSubnet: duplicateSubnetName,
				},
			}
			now := time.Now()
			afterNow := now.Add(1 * time.Minute)
			oldGNP.CreationTimestamp = v1.NewTime(now)
			test.paramSet.CreationTimestamp = v1.NewTime(afterNow)

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, oldGNP, v1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create existing GNP: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.paramSet, v1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create GKENetworkParamSet: %v", err)
			}

			// Wait for the conditions to be updated by the controller
			g.Eventually(func() (v1.Condition, error) {
				updatedParamSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, test.paramSet.Name, v1.GetOptions{})
				if err != nil {
					return v1.Condition{}, err
				}

				for _, condition := range updatedParamSet.Status.Conditions {
					if condition.Type == "Ready" {
						return condition, nil
					}
				}

				return v1.Condition{}, fmt.Errorf("GKENetworkParamSet Ready condition not found")
			}).Should(matchConditionIgnoringMessageAndLastTransitionTime(test.expectedCondition), "GKENetworkParamSet condition should match the expected condition")

		})
	}
}

func TestCrossValidateNetworkAndGnp(t *testing.T) {
	gkeNetworkParamSetName := "test-paramset"
	subnetName := "test-subnet"
	subnetSecondaryRangeName := "test-secondary-range"
	networkName := "network-name"

	tests := []struct {
		name              string
		network           *networkv1.Network
		paramSet          *networkv1.GKENetworkParamSet
		expectedCondition v1.Condition
	}{
		{
			name: "L3NetworkType with missing PodIPv4Ranges",
			network: &networkv1.Network{
				ObjectMeta: v1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			expectedCondition: v1.Condition{
				Type:   "ParamsReady",
				Status: v1.ConditionFalse,
				Reason: "L3SecondaryMissing",
			},
		},
		{
			name: "DeviceNetworkType with missing DeviceMode",
			network: &networkv1.Network{
				ObjectMeta: v1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			expectedCondition: v1.Condition{
				Type:   "ParamsReady",
				Status: v1.ConditionFalse,
				Reason: "DeviceModeMissing",
			},
		},
		{
			name: "Valid L3NetworkType",
			network: &networkv1.Network{
				ObjectMeta: v1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.L3NetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:           nonDefaultTestNetworkName,
					VPCSubnet:     subnetName,
					PodIPv4Ranges: &networkv1.SecondaryRanges{RangeNames: []string{subnetSecondaryRangeName}},
				},
			},
			expectedCondition: v1.Condition{
				Type:   "ParamsReady",
				Status: v1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "Valid DeviceNetworkType",
			network: &networkv1.Network{
				ObjectMeta: v1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			expectedCondition: v1.Condition{
				Type:   "ParamsReady",
				Status: v1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
		{
			name: "Valid and Network has mixed case kind in ParametersRef",
			network: &networkv1.Network{
				ObjectMeta: v1.ObjectMeta{
					Name: networkName,
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: "GkEnetworkparamSet"},
				},
			},
			paramSet: &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
					Name: gkeNetworkParamSetName,
				},
				Spec: networkv1.GKENetworkParamSetSpec{
					VPC:        nonDefaultTestNetworkName,
					VPCSubnet:  subnetName,
					DeviceMode: networkv1.NetDevice,
				},
			},
			expectedCondition: v1.Condition{
				Type:   "ParamsReady",
				Status: v1.ConditionTrue,
				Reason: "GNPParamsReady",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			testVals := setupGKENetworkParamSetController(ctx)

			subnetSecondaryCidr := "10.0.0.1/24"
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

			_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, test.network, v1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create Network: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, test.paramSet, v1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create GKENetworkParamSet: %v", err)
			}

			// Wait for the conditions to be updated by the controller
			g.Eventually(func() (v1.Condition, error) {
				updatedNetwork, err := testVals.networkClient.NetworkingV1().Networks().Get(ctx, test.network.Name, v1.GetOptions{})
				if err != nil {
					return v1.Condition{}, err
				}

				for _, condition := range updatedNetwork.Status.Conditions {
					if condition.Type == "ParamsReady" {
						return condition, nil
					}
				}

				return v1.Condition{}, fmt.Errorf("Network ParamsReady condition not found")
			}).Should(matchConditionIgnoringMessageAndLastTransitionTime(test.expectedCondition), "Network ParamsReady condition should match the expected condition")

		})
	}

}

func TestHandleGKENetworkParamSetDelete_NetworkInUse(t *testing.T) {

	tests := []struct {
		name            string
		networkUpdateFn func(ctx context.Context, networkName string, networkClient *fake.Clientset)
	}{
		{name: "Network no longer InUse",
			networkUpdateFn: func(ctx context.Context, networkName string, networkClient *fake.Clientset) {
				network, err := networkClient.NetworkingV1().Networks().Get(ctx, networkName, v1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get Network: %v", err)
				}
				// change Network to not in use
				network.SetAnnotations(map[string]string{})
				_, err = networkClient.NetworkingV1().Networks().Update(ctx, network, v1.UpdateOptions{})
				if err != nil {
					t.Fatalf("Failed to update Network status: %v", err)
				}
			},
		},
		{name: "Network deleted",
			networkUpdateFn: func(ctx context.Context, networkName string, networkClient *fake.Clientset) {
				err := networkClient.NetworkingV1().Networks().Delete(ctx, networkName, v1.DeleteOptions{})
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

			testVals := setupGKENetworkParamSetController(ctx)

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
				ObjectMeta: v1.ObjectMeta{
					Name: networkName,
					Annotations: map[string]string{
						networkv1.NetworkInUseAnnotationKey: networkv1.NetworkInUseAnnotationValTrue,
					},
				},
				Spec: networkv1.NetworkSpec{
					Type:          networkv1.DeviceNetworkType,
					ParametersRef: &networkv1.NetworkParametersReference{Name: gkeNetworkParamSetName, Kind: gnpKind},
				},
			}

			paramSet := &networkv1.GKENetworkParamSet{
				ObjectMeta: v1.ObjectMeta{
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

			_, err = testVals.networkClient.NetworkingV1().Networks().Create(ctx, network, v1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create Network: %v", err)
			}

			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Create(ctx, paramSet, v1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create GKENetworkParamSet: %v", err)
			}

			g.Consistently(func() (bool, error) {
				return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
			}).Should(gomega.BeTrue(), "finalizer should exist on GKENetworkParamSet")

			g.Eventually(func() bool {
				network, err := testVals.networkClient.NetworkingV1().Networks().Get(ctx, networkName, v1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get Network: %v", err)
				}

				return condmeta.IsStatusConditionTrue(network.Status.Conditions, "ParamsReady")
			}).Should(gomega.BeTrue(), "ParamsReady should be true in Network Conditions")

			newParamset, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get GKENetworkParamSet: %v", err)
			}

			// simulate a delete on GNP resource
			now := v1.Now()
			newParamset.SetDeletionTimestamp(&now)
			_, err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Update(ctx, newParamset, v1.UpdateOptions{})
			if err != nil {
				t.Fatalf("Failed to update GKENetworkParamSet: %v", err)
			}

			test.networkUpdateFn(ctx, networkName, testVals.networkClient)

			g.Eventually(func() (bool, error) {
				return testVals.doesGNPFinalizerExist(ctx, gkeNetworkParamSetName)
			}).Should(gomega.BeFalse(), "finalizer should be removed from GKENetworkParamSet")

			// The finalizer was removed, we need to manually handle the deletion
			err = testVals.networkClient.NetworkingV1().GKENetworkParamSets().Delete(ctx, gkeNetworkParamSetName, v1.DeleteOptions{})
			if err != nil {
				t.Fatalf("Failed to delete GKENetworkParamSet: %v", err)
			}

			// networkUpdateFn can delete network, so we only want to make an assertion
			// on network conditions if it still exists
			networkWasDeleted := false
			_, err = testVals.networkClient.NetworkingV1().Networks().Get(ctx, networkName, v1.GetOptions{})
			if errors.IsNotFound(err) {
				networkWasDeleted = true
			} else if err != nil {
				t.Fatalf("Failed to get Network: %v", err)
			}

			if !networkWasDeleted {
				g.Eventually(func() bool {
					network, err := testVals.networkClient.NetworkingV1().Networks().Get(ctx, networkName, v1.GetOptions{})
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
	paramSet, err := testVals.networkClient.NetworkingV1().GKENetworkParamSets().Get(ctx, gkeNetworkParamSetName, v1.GetOptions{})
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
