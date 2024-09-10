/*
Copyright 2024 The Kubernetes Authors.

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

package ccm

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"k8s.io/cloud-provider/app"
	fakecloud "k8s.io/cloud-provider/fake"
	"k8s.io/cloud-provider/names"
	cliflag "k8s.io/component-base/cli/flag"
	kubeapiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	kcmnames "k8s.io/kubernetes/cmd/kube-controller-manager/names"
	"k8s.io/kubernetes/pkg/controller/nodeipam/ipam"
	"k8s.io/kubernetes/test/integration/framework"
)

func Test_CloudControllerManagerGCP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start test api server.
	server := kubeapiservertesting.StartTestServerOrDie(t, nil, framework.DefaultTestServerFlags(), framework.SharedEtcd())
	defer server.TearDownFn()

	// Create client connecting to api server.
	client := clientset.NewForConfigOrDie(server.ClientConfig)

	// Create fake node
	_, err := client.CoreV1().Nodes().Create(ctx, makeNode("node0"), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Node %v", err)
	}

	kubeconfig := createKubeconfigFileForRestConfig(server.ClientConfig)
	// nolint:errcheck // Ignore the error trying to delete the kubeconfig file used for the test
	defer os.Remove(kubeconfig)
	args := []string{
		"--kubeconfig=" + kubeconfig,
		"--cloud-provider=fakeCloud",
		"--configure-cloud-routes=false",
		// Following flags necessary for GCP NodeIPAMController
		"--cidr-allocator-type=" + string(ipam.RangeAllocatorType),
		"--allocate-node-cidrs=true",
		"--cluster-cidr=10.0.0.0/16",
	}
	// Hard-code values for the cloud provider.
	fakeCloud := &fakecloud.Cloud{
		Zone: cloudprovider.Zone{
			FailureDomain: "zone-0",
			Region:        "region-1",
		},
		EnableInstancesV2:  true,
		ExistsByProviderID: true,
		ProviderID: map[types.NodeName]string{
			types.NodeName("node0"): "12345",
		},
		InstanceTypes: map[types.NodeName]string{
			types.NodeName("node0"): "t1.micro",
		},
		ExtID: map[types.NodeName]string{
			types.NodeName("node0"): "12345",
		},
		Addresses: []v1.NodeAddress{
			{
				Type:    v1.NodeHostName,
				Address: "node0.cloud.internal",
			},
			{
				Type:    v1.NodeInternalIP,
				Address: "10.0.0.1",
			},
			{
				Type:    v1.NodeExternalIP,
				Address: "132.143.154.163",
			},
		},
		ErrByProviderID: nil,
		Err:             nil,
	}

	// Register fake GCE cloud provider
	cloudprovider.RegisterCloudProvider(
		"fakeCloud",
		func(config io.Reader) (cloudprovider.Interface, error) {
			return fakeCloud, nil
		})

	// Add the GCP-specific NodeIPAMController to the cloud-controller-manager.
	controllerInitializers := app.DefaultInitFuncConstructors
	nodeIpamController := nodeIPAMController{}
	nodeIpamController.nodeIPAMControllerOptions.NodeIPAMControllerConfiguration = &nodeIpamController.nodeIPAMControllerConfiguration
	fss := cliflag.NamedFlagSets{}
	nodeIpamController.nodeIPAMControllerOptions.AddFlags(fss.FlagSet("nodeipam controller"))
	controllerInitializers[kcmnames.NodeIpamController] = app.ControllerInitFuncConstructor{
		Constructor: nodeIpamController.startNodeIpamControllerWrapper,
	}

	//app.ControllersDisabledByDefault.Insert(kcmnames.NodeIpamController)
	aliasMap := names.CCMControllerAliases()
	aliasMap["nodeipam"] = kcmnames.NodeIpamController

	// Start the test cloud-controller-manager.
	ccm, err := StartTestServerWithOptions(context.Background(), args, controllerInitializers, aliasMap)
	if err != nil {
		panic(fmt.Errorf("failed to launch test ccm server: %v", err))
	}
	defer ccm.TearDownFn()

	// There should be only the taint TaintNodeNotReady, added by the admission plugin TaintNodesByCondition
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 50*time.Second, true, func(ctx context.Context) (done bool, err error) {
		n, err := client.CoreV1().Nodes().Get(ctx, "node0", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(n.Spec.Taints) != 1 {
			return false, nil
		}
		if n.Spec.Taints[0].Key != v1.TaintNodeNotReady {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Logf("Fake Cloud Provider calls: %v", fakeCloud.Calls)
		t.Fatalf("expected node to not have Taint: %v", err)
	}

	// Validate the Zone/Region labels have been set on the Node.
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 50*time.Second, true, func(ctx context.Context) (done bool, err error) {
		n, err := client.CoreV1().Nodes().Get(ctx, "node0", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if n.Labels[v1.LabelFailureDomainBetaZone] == "zone-0" &&
			n.Labels[v1.LabelFailureDomainBetaRegion] == "region-1" &&
			n.Labels[v1.LabelTopologyZone] == "zone-0" &&
			n.Labels[v1.LabelTopologyRegion] == "region-1" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Logf("Fake Cloud Provider calls: %v", fakeCloud.Calls)
		t.Fatalf("expected node to have all zone/region labels: %v", err)
	}

	// Validate the ProviderID has been set on the Node.
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 50*time.Second, true, func(ctx context.Context) (done bool, err error) {
		n, err := client.CoreV1().Nodes().Get(ctx, "node0", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if n.Spec.ProviderID == "12345" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Logf("Fake Cloud Provider calls: %v", fakeCloud.Calls)
		t.Fatalf("expected node to ProviderID: %v", err)
	}

	// Validate the NodeIPAMController sets the PodCIDR on the Node.
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 50*time.Second, true, func(ctx context.Context) (done bool, err error) {
		n, err := client.CoreV1().Nodes().Get(ctx, "node0", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if n.Spec.PodCIDR == "10.0.0.0/24" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Logf("Fake Cloud Provider calls: %v", fakeCloud.Calls)
		t.Fatalf("expected node to have pod cidr: %v", err)
	}
}

// sigs.k8s.io/controller-runtime/pkg/envtest
func createKubeconfigFileForRestConfig(restConfig *rest.Config) string {
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["default-cluster"] = &clientcmdapi.Cluster{
		Server:                   restConfig.Host,
		TLSServerName:            restConfig.ServerName,
		CertificateAuthorityData: restConfig.CAData,
	}
	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:  "default-cluster",
		AuthInfo: "default-user",
	}
	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	authinfos["default-user"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: restConfig.CertData,
		ClientKeyData:         restConfig.KeyData,
		Token:                 restConfig.BearerToken,
	}
	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authinfos,
	}
	kubeConfigFile, _ := os.CreateTemp("", "kubeconfig")
	_ = clientcmd.WriteToFile(clientConfig, kubeConfigFile.Name())
	return kubeConfigFile.Name()
}

func makeNode(name string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{{
				Key:    cloudproviderapi.TaintExternalCloudProvider,
				Value:  "true",
				Effect: v1.TaintEffectNoSchedule,
			}},
			Unschedulable: false,
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:              v1.NodeReady,
					Status:            v1.ConditionUnknown,
					LastHeartbeatTime: metav1.Time{Time: time.Now()},
				},
			},
		},
	}
}
