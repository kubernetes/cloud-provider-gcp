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
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/spf13/pflag"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/klog/v2"
	nodeipamcontrolleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	nodeipamconfig "k8s.io/kubernetes/pkg/controller/nodeipam/config"

	_ "k8s.io/component-base/metrics/prometheus/version" // for version metric registration
	// NOTE: Importing all in-tree cloud-providers is not required when
	// implementing an out-of-tree cloud-provider.
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/legacy-cloud-providers/gce"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	c, err := s.Config([]string{}, app.ControllersDisabledByDefault.List())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// initialize cloud provider with the cloud provider name and config file provided
	cloudconfig := c.ComponentConfig.KubeCloudShared.CloudProvider
	cloud, err := cloudprovider.InitCloudProvider(cloudconfig.Name, cloudconfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if c.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}

	// Initialize the cloud provider with a reference to the clientBuilder
	cloud.Initialize(c.ClientBuilder, make(chan struct{}))
	// Set the informer on the user cloud object
	if informerUserCloud, ok := cloud.(cloudprovider.InformerUser); ok {
		informerUserCloud.SetInformers(c.SharedInformers)
	}

	controllerInitializers := app.DefaultControllerInitializers(c.Complete(), cloud)

	fs := pflag.NewFlagSet("fs", pflag.ContinueOnError)
	var nodeIPAMControllerOptions nodeipamcontrolleroptions.NodeIPAMControllerOptions
	nodeIPAMControllerOptions.AddFlags(fs)
	errors := nodeIPAMControllerOptions.Validate()
	if len(errors) > 0 {
		klog.Fatal("NodeIPAM controller values are not properly.")
	}
	var nodeipamconfig nodeipamconfig.NodeIPAMControllerConfiguration
	nodeIPAMControllerOptions.ApplyTo(&nodeipamconfig)

	controllerInitializers["nodeipam"] = startNodeIpamControllerWrapper(c.Complete(), nodeipamconfig, cloud)

	command := app.NewCloudControllerManagerCommand(s, c, controllerInitializers)

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	// Here is an sample
	pflag.CommandLine.SetNormalizeFunc(flag.WordSepNormalizeFunc)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func startNodeIpamControllerWrapper(ccmconfig *cloudcontrollerconfig.CompletedConfig, nodeipamconfig nodeipamconfig.NodeIPAMControllerConfiguration, cloud cloudprovider.Interface) func(ctx genericcontrollermanager.ControllerContext) (http.Handler, bool, error) {
	return func(ctx genericcontrollermanager.ControllerContext) (http.Handler, bool, error) {
		return startNodeIpamController(ccmconfig, nodeipamconfig, ctx, cloud)
	}
}
