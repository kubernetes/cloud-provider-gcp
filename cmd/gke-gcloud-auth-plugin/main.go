package main

import (
	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-gcp/pkg/clientgocred"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
)

var (
	useAdcPtr = pflag.Bool("use_application_default_credentials", false, "Output is an ExecCredential filled with application default credentials.")
)

func main() {
	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	opts := &clientgocred.Options{UseApplicationDefaultCredentials: *useAdcPtr}
	if err := clientgocred.PrintCred(opts); err != nil {
		klog.Fatalf("Print credential failed with error :%v", err)
	}
}
