/*
Copyright 2016 The Kubernetes Authors.

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

// The external controller manager is responsible for running controller loops that
// are cloud provider dependent. It uses the API to listen to new events on resources.

package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-gcp/providers/gce"
	_ "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/names"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
	kcmnames "k8s.io/kubernetes/cmd/kube-controller-manager/names"
)

const (
	gkeServiceLBControllerName     = "gke-service-lb-controller"
	gkeServiceControllerClientName = "gke-service-controller"
	gkeServiceAlias                = "gke-service"
)

// enableMultiProject is bound to a command-line flag. When true, it enables the
// projectFromNodeProviderID option of the GCE cloud provider, instructing it to
// use the project specified in the Node's providerID for GCE API calls.
//
// This flag should only be enabled when the Node's providerID can be fully
// trusted.
//
// Flag binding occurs in main()
var enableMultiProject bool

// enableDiscretePortForwarding is bound to a command-line flag. It enables
// the same option of the GCE cloud provider to forward individual ports
// instead of port ranges in Forwarding Rules for external load balancers.
var enableDiscretePortForwarding bool

// enableRBSDefaultForGCEL4NetLB is bound to a command-line flag. It enables
// the option to default L4 NetLB to RBS, only controlling NetLB services with
// LoadBalancerClass
var enableRBSDefaultForL4NetLB bool

// enableL4LBAnnotations is bound to a command-line flag. It enables
// the controller to write annotations related to the provisioned resources
// for L4 Load Balancers services
var enableL4LBAnnotations bool

func main() {
	rand.Seed(time.Now().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	ccmOptions, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	controllerInitializers := app.DefaultInitFuncConstructors

	fss := cliflag.NamedFlagSets{}

	cloudProviderFS := fss.FlagSet("GCE Cloud Provider")
	cloudProviderFS.BoolVar(&enableMultiProject, "enable-multi-project", false, "Enables project selection from Node providerID for GCE API calls. CAUTION: Only enable if Node providerID is configured by a trusted source.")
	cloudProviderFS.BoolVar(&enableDiscretePortForwarding, "enable-discrete-port-forwarding", false, "Enables forwarding of individual ports instead of port ranges for GCE external load balancers.")
	cloudProviderFS.BoolVar(&enableRBSDefaultForL4NetLB, "enable-rbs-default-l4-netlb", false, "Enables RBS defaulting for GCE L4 NetLB")
	cloudProviderFS.BoolVar(&enableL4LBAnnotations, "enable-l4-lb-annotations", false, "Enables Annotations for GCE L4 LB Services")

	// add new controllers and initializers
	nodeIpamController := nodeIPAMController{}
	nodeIpamController.nodeIPAMControllerOptions.NodeIPAMControllerConfiguration = &nodeIpamController.nodeIPAMControllerConfiguration
	nodeIpamController.nodeIPAMControllerOptions.AddFlags(fss.FlagSet("nodeipam controller"))
	controllerInitializers[kcmnames.NodeIpamController] = app.ControllerInitFuncConstructor{
		Constructor: nodeIpamController.startNodeIpamControllerWrapper,
	}

	controllerInitializers["gkenetworkparamset"] = app.ControllerInitFuncConstructor{
		Constructor: startGkeNetworkParamSetControllerWrapper,
	}

	controllerInitializers[gkeServiceLBControllerName] = app.ControllerInitFuncConstructor{
		InitContext: app.ControllerInitContext{
			ClientName: gkeServiceControllerClientName,
		},
		Constructor: startGkeServiceControllerWrapper,
	}

	// add controllers disabled by default
	app.ControllersDisabledByDefault.Insert("gkenetworkparamset")
	app.ControllersDisabledByDefault.Insert(gkeServiceLBControllerName)
	aliasMap := names.CCMControllerAliases()
	aliasMap["nodeipam"] = kcmnames.NodeIpamController
	aliasMap[gkeServiceAlias] = gkeServiceLBControllerName
	command := app.NewCloudControllerManagerCommand(ccmOptions, cloudInitializer, controllerInitializers, aliasMap, fss, wait.NeverStop)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func cloudInitializer(config *config.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider

	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider with name: %v and configFile: %v could not be initialized: %v", cloudConfig.Name, cloudConfig.CloudConfigFile, err)
	}
	if cloud == nil {
		klog.Fatalf("Cloud provider with name: %v and configFile: %v is nil", cloudConfig.Name, cloudConfig.CloudConfigFile)
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}

	if enableMultiProject {
		gceCloud, ok := (cloud).(*gce.Cloud)
		if !ok {
			// Fail-fast: If enableMultiProject is set, the cloud provider MUST
			// be GCE. A non-GCE provider indicates a misconfiguration. Ideally,
			// we never expect this to be executed.
			klog.Fatalf("multi-project mode requires GCE cloud provider, but got %T", cloud)
		}
		gceCloud.SetProjectFromNodeProviderID(true)
	}

	if enableDiscretePortForwarding {
		gceCloud, ok := (cloud).(*gce.Cloud)
		if !ok {
			// Fail-fast: If enableDiscretePortForwarding is set, the cloud
			// provider MUST be GCE.
			klog.Fatalf("enable-discrete-port-forwarding requires GCE cloud provider, but got %T", cloud)
		}
		gceCloud.SetEnableDiscretePortForwarding(true)
	}

	if enableRBSDefaultForL4NetLB {
		gceCloud, ok := (cloud).(*gce.Cloud)
		if !ok {
			// Fail-fast: If enableRBSDefaultForL4NetLB is set, the cloud
			// provider MUST be GCE.
			klog.Fatalf("enable-rbs-default-l4-netlb requires GCE cloud provider, but got %T", cloud)
		}
		gceCloud.SetEnableRBSDefaultForL4NetLB(true)
	}

	if enableL4LBAnnotations {
		gceCloud, ok := (cloud).(*gce.Cloud)
		if !ok {
			// Fail-fast: If enableL4LBAnnotations is set, the cloud
			// provider MUST be GCE.
			klog.Fatalf("enable-l4-lb-annotations requires GCE cloud provider, but got %T", cloud)
		}
		gceCloud.SetEnableL4LBAnnotations(true)
	}

	return cloud
}
