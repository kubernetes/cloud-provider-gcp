package ipam

import (
	"context"
	"testing"

	ntv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1"
	ntclient "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	ntfakeclient "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cloud-provider-gcp/providers/gce"
)

// Mock the utilnode.NodePoolSubnetLabelPrefix for testing purposes
const testNodePoolSubnetLabelPrefix = "cloud.google.com/gke-node-pool-subnet"

func TestGetNodeSubnetLabel(t *testing.T) {
	tests := []struct {
		name       string
		node       *v1.Node
		wantFound  bool
		wantSubnet string
	}{
		{
			name: "node has subnet label",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "test-subnet-a",
						"another-label":               "value",
					},
				},
			},
			wantFound:  true,
			wantSubnet: "test-subnet-a",
		},
		{
			name: "node has subnet label with empty value",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "",
					},
				},
			},
			wantFound:  true,
			wantSubnet: "",
		},
		{
			name: "node does not have subnet label",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"another-label": "value",
					},
				},
			},
			wantFound:  false,
			wantSubnet: "",
		},
		{
			name: "node has no labels",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{},
			},
			wantFound:  false,
			wantSubnet: "",
		},
		{
			name: "node has label with similar prefix but different",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix + "-extra": "some-value",
					},
				},
			},
			wantFound:  false,
			wantSubnet: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotFound, gotSubnet := getNodeSubnetLabel(tc.node)
			if gotFound != tc.wantFound {
				t.Errorf("getNodeSubnetLabel() gotFound = %v, want %v", gotFound, tc.wantFound)
			}
			if gotSubnet != tc.wantSubnet {
				t.Errorf("getNodeSubnetLabel() gotSubnet = %v, want %v", gotSubnet, tc.wantSubnet)
			}
		})
	}
}

func TestGetSubnetWithPrefixFromURL(t *testing.T) {

	tests := []struct {
		name             string
		url              string
		wantSubnetName   string
		wantSubnetPrefix string
		wantErr          bool
	}{
		{
			name:             "valid subnet URL",
			url:              "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/subnetworks/my-subnet",
			wantSubnetName:   "my-subnet",
			wantSubnetPrefix: "projects/my-project/regions/us-central1/subnetworks/",
			wantErr:          false,
		},
		{
			name:             "URL without https prefix",
			url:              "projects/another-project/regions/europe-west1/subnetworks/another-subnet",
			wantSubnetName:   "another-subnet",
			wantSubnetPrefix: "projects/another-project/regions/europe-west1/subnetworks/",
			wantErr:          false,
		},
		{
			name:    "URL missing 'projects/'",
			url:     "https://www.googleapis.com/compute/v1/my-project/regions/us-central1/subnetworks/my-subnet",
			wantErr: true,
		},
		{
			name:    "Incorrect URL path, too short",
			url:     "projects",
			wantErr: true,
		},
		{
			name:    "URL with 'subnetworks/' but no project parts",
			url:     "https://www.googleapis.com/compute/v1/regions/us-central1/subnetworks/my-subnet",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSubnetName, gotSubnetPrefix, err := getSubnetWithPrefixFromURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Errorf("getSubnetWithPrefixFromURL() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if gotSubnetName != tc.wantSubnetName {
				t.Errorf("getSubnetWithPrefixFromURL() gotSubnetName = %v, want %v", gotSubnetName, tc.wantSubnetName)
			}
			if gotSubnetPrefix != tc.wantSubnetPrefix {
				t.Errorf("getSubnetWithPrefixFromURL() gotSubnetPrefix = %v, want %v", gotSubnetPrefix, tc.wantSubnetPrefix)
			}

		})
	}
}

func TestUpdateNodeTopology(t *testing.T) {

	testClusterValues := gce.DefaultTestClusterValues()
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	emptyNodeTopologyCR := &ntv1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		// Status is initially left empty
	}
	ntClient := ntfakeclient.NewSimpleClientset(emptyNodeTopologyCR)

	tests := []struct {
		name            string
		node            *v1.Node
		existingSubnets []string
		wantSubnets     []string
	}{
		{
			name: "node's subnet already exists in the cr",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet-def",
						"another-label":               "value",
					},
				},
			},
			existingSubnets: []string{"subnet-def"},
			wantSubnets:     []string{"subnet-def"},
		},
		{
			name: "node has a new subnet",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "new-subnet",
					},
				},
			},
			existingSubnets: []string{"subnet-def"},
			wantSubnets:     []string{"subnet-def", "new-subnet"},
		},
		{
			name: "node has a subnet and cr's subnets is empty",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet-def",
					},
				},
			},
			existingSubnets: []string{},
			wantSubnets:     []string{"subnet-def"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// populate the def subnet URL
			// pass the fake nodetopology client
			// create a fake default cr (outside this loop. at the begining of the test)
			// populate the existingSubnets in the cr
			// call the ca.updateNodeTopology and read the subnets
			// verify the subnets with wantSubnets
			addSubnetsToCR(tc.existingSubnets, ntClient)
			ca := &cloudCIDRAllocator{
				cloud:              fakeGCE,
				nodeTopologyClient: ntClient,
			}
			err := ca.updateNodeTopology(tc.node)
			if err != nil {
				t.Errorf("ca.updateNodeTopology() returned error: %v", err)
			}
			if !verifySubnetsInCR(t, tc.wantSubnets, ntClient) {
				t.Errorf("ca.updateNodeTopology() returned incorrect subnets")
			}
		})
	}
}

func addSubnetsToCR(subnets []string, client ntclient.Interface) {
	ctx := context.Background()

	cr := &ntv1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		// Status is initially empty
	}
	for _, subnet := range subnets {
		cr.Status.Subnets = append(cr.Status.Subnets, ntv1.SubnetConfig{
			Name: subnet,
			// We are ignoring the path field here on purpose.
			// It is tested separately on it's own.
		})
	}
	client.NetworkingV1().NodeTopologies().UpdateStatus(ctx, cr, metav1.UpdateOptions{})
}

func verifySubnetsInCR(t *testing.T, subnets []string, client ntclient.Interface) bool {
	ctx := context.Background()
	hm := make(map[string]bool)
	for _, subnet := range subnets {
		hm[subnet] = true
	}

	cr, _ := client.NetworkingV1().NodeTopologies().Get(ctx, "default", metav1.GetOptions{})

	for _, subnetConfig := range cr.Status.Subnets {
		if _, found := hm[subnetConfig.Name]; !found {
			return false
		}
	}
	return true
}
