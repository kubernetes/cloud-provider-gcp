/*
Copyright 2017 The Kubernetes Authors.

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

// The GKE certificates controller is responsible for monitoring certificate
// signing requests and (potentially) auto-approving and signing them within
// GKE.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"

	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/app"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	"k8s.io/kubernetes/pkg/version/verflag"
)

var metricsPort = pflag.Int("metrics-port", 8089, "Port to expose Prometheus metrics on")

func main() {
	s := app.NewGCPControllerManager()
	s.AddFlags(pflag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	logs.InitLogs()
	defer logs.FlushLogs()

	verflag.PrintAndExitIfRequested()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *metricsPort), nil))
	}()

	if err := app.Run(s); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
