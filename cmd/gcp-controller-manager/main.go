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
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Register GCP auth provider plugin.
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/version/verflag"
)

const (
	leaderElectionResourceLockNamespace = "kube-system"
	leaderElectionResourceLockName      = "gcp-controller-manager"
)

var (
	metricsPort                        = pflag.Int("metrics-port", 8089, "Port to expose Prometheus metrics on")
	kubeconfig                         = pflag.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information.")
	clusterSigningGKEKubeconfig        = pflag.String("cluster-signing-gke-kubeconfig", "", "If set, use the kubeconfig file to call GKE to sign cluster-scoped certificates instead of using a local private key.")
	gceConfigPath                      = pflag.String("gce-config", "/etc/gce.conf", "Path to gce.conf.")
	controllers                        = pflag.StringSlice("controllers", []string{"*"}, "Controllers to enable. Possible controllers are: "+strings.Join(loopNames(), ",")+".")
	csrApproverVerifyClusterMembership = pflag.Bool("csr-validate-cluster-membership", true, "Validate that VMs requesting CSRs belong to current GKE cluster.")
	csrApproverAllowLegacyKubelet      = pflag.Bool("csr-allow-legacy-kubelet", true, "Allow legacy kubelet bootstrap flow.")
)

func main() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	leConfig := &componentbaseconfig.LeaderElectionConfiguration{
		LeaderElect:   true,
		LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
		RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
		RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
		ResourceLock:  rl.EndpointsResourceLock,
	}
	leaderelectionconfig.BindFlags(leConfig, pflag.CommandLine)

	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	s := &controllerManager{
		clusterSigningGKEKubeconfig:        *clusterSigningGKEKubeconfig,
		gceConfigPath:                      *gceConfigPath,
		controllers:                        *controllers,
		csrApproverVerifyClusterMembership: *csrApproverVerifyClusterMembership,
		csrApproverAllowLegacyKubelet:      *csrApproverAllowLegacyKubelet,
		leaderElectionConfig:               *leConfig,
	}
	var err error
	s.kubeconfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Exitf("failed loading kubeconfig: %v", err)
	}
	// bump the QPS limits per controller up from defaults of 5 qps / 10 burst
	s.kubeconfig.QPS = 100
	s.kubeconfig.Burst = 200

	s.gcpConfig, err = loadGCPConfig(s.gceConfigPath)
	if err != nil {
		klog.Exitf("failed loading GCP config: %v", err)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		klog.Exit(http.ListenAndServe(fmt.Sprintf(":%d", *metricsPort), nil))
	}()

	if err := run(s); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// controllerManager is the main context object for the package.
type controllerManager struct {
	// Fields initialized from flags.
	clusterSigningGKEKubeconfig        string
	gceConfigPath                      string
	controllers                        []string
	csrApproverVerifyClusterMembership bool
	csrApproverAllowLegacyKubelet      bool
	leaderElectionConfig               componentbaseconfig.LeaderElectionConfiguration

	// Fields initialized from other sources.
	gcpConfig  gcpConfig
	kubeconfig *restclient.Config
}

func (s *controllerManager) isEnabled(name string) bool {
	var star bool
	for _, controller := range s.controllers {
		if controller == name {
			return true
		}
		if controller == "-"+name {
			return false
		}
		if controller == "*" {
			star = true
		}
	}
	return star
}

// run runs the controllerManager. This should never exit.
func run(s *controllerManager) error {
	ctx := context.Background()

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: s.kubeconfig}

	informerClient := clientBuilder.ClientOrDie("gcp-controller-manager-shared-informer")
	sharedInformers := informers.NewSharedInformerFactory(informerClient, time.Duration(12)*time.Hour)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: v1core.New(clientBuilder.ClientOrDie("gcp-controller-manager").CoreV1().RESTClient()).Events(""),
	})

	run := func(ctx context.Context) {
		for name, loop := range loops() {
			if !s.isEnabled(name) {
				continue
			}
			name = "gcp-" + name
			loopClient, err := clientBuilder.Client(name)
			if err != nil {
				klog.Fatalf("failed to start client for %q: %v", name, err)
			}
			if loop(&controllerContext{
				client:          loopClient,
				sharedInformers: sharedInformers,
				recorder: eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{
					Component: name,
				}),
				gcpCfg:                             s.gcpConfig,
				clusterSigningGKEKubeconfig:        s.clusterSigningGKEKubeconfig,
				csrApproverVerifyClusterMembership: s.csrApproverVerifyClusterMembership,
				csrApproverAllowLegacyKubelet:      s.csrApproverAllowLegacyKubelet,
				done:                               ctx.Done(),
			}); err != nil {
				klog.Fatalf("Failed to start %q: %v", name, err)
			}
		}
		sharedInformers.Start(ctx.Done())
		<-ctx.Done()
	}

	if s.leaderElectionConfig.LeaderElect {
		leaderElectionClient, err := clientset.NewForConfig(restclient.AddUserAgent(s.kubeconfig, "leader-election"))
		if err != nil {
			return err
		}
		leaderElectionConfig, err := makeLeaderElectionConfig(s.leaderElectionConfig, leaderElectionClient, eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{
			Component: "gcp-controller-manager-leader-election",
		}))
		if err != nil {
			return err
		}
		leaderElectionConfig.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("lost leader election, exiting")
			},
		}

		leaderElector, err := leaderelection.NewLeaderElector(*leaderElectionConfig)
		if err != nil {
			return err
		}
		leaderElector.Run(ctx)
		panic("unreachable")
	}

	run(nil)
	return fmt.Errorf("should never reach this point")
}

func makeLeaderElectionConfig(config componentbaseconfig.LeaderElectionConfiguration, client clientset.Interface, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	rl, err := resourcelock.New(
		config.ResourceLock,
		leaderElectionResourceLockNamespace,
		leaderElectionResourceLockName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      hostname,
			EventRecorder: recorder,
		})
	if err != nil {
		return nil, fmt.Errorf("couldn't create resource lock: %v", err)
	}
	return &leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: config.LeaseDuration.Duration,
		RenewDeadline: config.RenewDeadline.Duration,
		RetryPeriod:   config.RetryPeriod.Duration,
	}, nil
}
