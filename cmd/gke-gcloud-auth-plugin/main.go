package main

import (
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-gcp/pkg/clientgocred"
	"k8s.io/component-base/version/verflag"
)

var (
	useAdcPtr = pflag.Bool("use_application_default_credentials", false, "Output is an ExecCredential filled with application default credentials.")
)

func main() {
	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	opts := &clientgocred.Options{UseApplicationDefaultCredentials: *useAdcPtr}
	if err := clientgocred.PrintCred(opts); err != nil {
		panic(fmt.Errorf("print credential failed with error: %w", err))
	}
}
