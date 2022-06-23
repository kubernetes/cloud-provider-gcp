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
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine) // this is required to setup klog flags
	pflag.Parse()

	verflag.PrintAndExitIfRequested()

	opts := &cred.Options{UseApplicationDefaultCredentials: *useApplicationDefaultCredentials}
	if err := cred.PrintCred(opts); err != nil {
		klog.Exit(fmt.Errorf("print credential failed with error: %w", err))
	}
}
