package ipam

import (
	"context"
	"testing"
	"time"

	ntv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1"
	ntclient "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	ntfakeclient "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/cloud-provider-gcp/providers/gce"
)

// Mock the utilnode.NodePoolSubnetLabelPrefix for testing purposes
const (
	testNodePoolSubnetLabelPrefix = "cloud.google.com/gke-node-pool-subnet"
	exampleSubnetURL              = "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/subnetworks/subnet-def"
	exampleSubnetPathPrefix       = "projects/my-project/regions/us-central1/subnetworks/"
)

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

func testClient() *ntfakeclient.Clientset {
	emptyNodeTopologyCR := &ntv1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	return ntfakeclient.NewSimpleClientset(emptyNodeTopologyCR)
}

func TestNodeTopologySync(t *testing.T) {
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.SubnetworkURL = exampleSubnetURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)
	ntClient := testClient()

	tests := []struct {
		name            string
		node            *v1.Node
		nodeListInCache []*v1.Node
		existingSubnets []string
		wantSubnets     []string
		wantErr         bool
	}{
		{
			name: "node's subnet already exists in the cr",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a-node",
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "a-node",
						Labels: map[string]string{
							testNodePoolSubnetLabelPrefix: "subnet-def",
							"another-label":               "value",
						},
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
					Name: "a-new-node",
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "a-new-node",
						Labels: map[string]string{
							testNodePoolSubnetLabelPrefix: "new-subnet",
						},
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
					Name: "a-node",
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "a-node",
						Labels: map[string]string{
							testNodePoolSubnetLabelPrefix: "subnet-def",
						},
					},
				},
			},
			existingSubnets: []string{},
			wantSubnets:     []string{"subnet-def"},
		},
		{
			name: "node has no subnet label and ensure we add the default subnet to cr",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "a-node",
					Labels: map[string]string{},
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "a-node",
						Labels: map[string]string{},
					},
				},
			},
			existingSubnets: []string{},
			wantSubnets:     []string{"subnet-def"},
		},
		{
			name: "delete node is reconciliation - delete default node",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-exist-node",
				},
			},
			nodeListInCache: []*v1.Node{},
			existingSubnets: []string{"subnet-def"},
			wantSubnets:     []string{"subnet-def"},
		},
		{
			name: "delete node is reconciliation - delete msc node",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-exist-node",
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
			existingSubnets: []string{"new-subnet"},
			wantSubnets:     []string{"subnet-def"},
		},
		{
			name: "delete node is reconciliation - delete one msc node among multiple msc nodes",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-exist-node",
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "additional-subnet-1",
						Labels: map[string]string{
							testNodePoolSubnetLabelPrefix: "remaining-subnet",
							"another-label":               "value",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "additional-subnet-2",
						Labels: map[string]string{
							testNodePoolSubnetLabelPrefix: "remaining-subnet-2",
							"another-label":               "value2",
						},
					},
				},
			},
			existingSubnets: []string{"new-subnet", "remaining-subnet", "remaining-subnet-2"},
			wantSubnets:     []string{"subnet-def", "remaining-subnet", "remaining-subnet-2"},
		},
		{
			name: "delete node is reconciliation - delete all msc nodes",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-exist-node",
				},
			},
			nodeListInCache: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "random-subnet",
						Labels: map[string]string{
							"random-label": "remaining-subnet",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "default-subnet",
						Labels: map[string]string{},
					},
				},
			},
			existingSubnets: []string{},
			wantSubnets:     []string{"subnet-def"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addSubnetsToCR(tc.existingSubnets, ntClient)
			fakeClient := &fake.Clientset{}
			fakeInformerFactory := informers.NewSharedInformerFactory(fakeClient, 0*time.Second)
			fakeNodeInformer := fakeInformerFactory.Core().V1().Nodes()
			for _, node := range tc.nodeListInCache {
				fakeNodeInformer.Informer().GetStore().Add(node)
			}
			syncer := &NodeTopologySyncer{
				cloud:              fakeGCE,
				nodeTopologyClient: ntClient,
				nodeLister:         fakeNodeInformer.Lister(),
			}
			key, _ := nodeTopologyKeyFun(tc.node)
			err := syncer.sync(key)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NodeTopologySyncer.sync() returns nil but want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NodeTopologySyncer.sync() returned error: %v", err)
			}
			if ok, cr := verifySubnetsInCR(t, tc.wantSubnets, ntClient); !ok {
				t.Errorf("NodeTopologySyncer.sync() returned incorrect subnets, got %v, expected %v", cr.Status.Subnets, tc.wantSubnets)
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
			Name:       subnet,
			SubnetPath: exampleSubnetPathPrefix + subnet,
		})
	}
	client.NetworkingV1().NodeTopologies().UpdateStatus(ctx, cr, metav1.UpdateOptions{})
}

func verifySubnetsInCR(t *testing.T, subnets []string, client ntclient.Interface) (bool, *ntv1.NodeTopology) {
	ctx := context.Background()
	hm := make(map[string]bool)
	for _, subnet := range subnets {
		hm[subnet] = true
	}

	cr, _ := client.NetworkingV1().NodeTopologies().Get(ctx, "default", metav1.GetOptions{})

	if len(subnets) != len(cr.Status.Subnets) {
		return false, cr
	}
	for _, subnetConfig := range cr.Status.Subnets {
		if _, found := hm[subnetConfig.Name]; !found {
			return false, cr
		}
		if subnetConfig.SubnetPath != (exampleSubnetPathPrefix + subnetConfig.Name) {
			return false, cr
		}
	}
	return true, nil
}
