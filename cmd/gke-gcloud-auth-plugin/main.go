package main

import (
	"flag"
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin/cred"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
)

var (
	useApplicationDefaultCredentials = pflag.Bool("use_application_default_credentials", false, "Output is an ExecCredential filled with application default credentials.")
	useEdgeCloud					 = pflag.Bool("use_edge_cloud", false, "Output is an ExecCredential for an edge cloud cluster.")
	location						 = pflag.String("location", "", "Location of the Cluster.")
	cluster							 = pflag.String("cluster", "", "Name of the Cluster.")
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine) // this is required to setup klog flags
	pflag.Parse()

	verflag.PrintAndExitIfRequested()
	validateFlags()

	var edgeCloudOpts *cred.EdgeCloudOptions = nil
	if *useEdgeCloud {
		edgeCloudOpts = &cred.EdgeCloudOptions{Location: *location, ClusterName: *cluster}
	}

	opts := &cred.Options{
		UseApplicationDefaultCredentials: *useApplicationDefaultCredentials,
		EdgeCloud: edgeCloudOpts,
	}

	if err := cred.PrintCred(opts); err != nil {
		klog.Exit(fmt.Errorf("print credential failed with error: %w", err))
	}
}

func validateFlags() {
	if *useEdgeCloud {
		if *useApplicationDefaultCredentials {
			klog.Exit(fmt.Errorf("For --use_edge_cloud: application default credentials are not compatible."))
		}
		if *location == "" || *cluster == "" {
			klog.Exit(fmt.Errorf("For --use_edge_cloud: --location and --cluster are required."))
		}
	}
}
