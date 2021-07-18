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
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/healthz"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/version/verflag"
	"k8s.io/controller-manager/pkg/clientbuilder"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

const (
	leaderElectionResourceLockNamespace = "kube-system"
	leaderElectionResourceLockName      = "gcp-controller-manager"

	kubeconfigQPS     = 100
	kubeconfigBurst   = 200
	kubeconfigTimeout = 30 * time.Second
)

var (
	port                               = pflag.Int("port", 8089, "Port to serve status endpoints on (such as /healthz and /metrics).")
	metricsPort                        = pflag.Int("metrics-port", 8089, "Deprecated. Port to expose Prometheus metrics on. If not set, uses the value of --port.")
	kubeconfig                         = pflag.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information.")
	clusterSigningGKEKubeconfig        = pflag.String("cluster-signing-gke-kubeconfig", "", "If set, use the kubeconfig file to call GKE to sign cluster-scoped certificates instead of using a local private key.")
	gceConfigPath                      = pflag.String("gce-config", "/etc/gce.conf", "Path to gce.conf.")
	controllers                        = pflag.StringSlice("controllers", []string{"*"}, "Controllers to enable. Possible controllers are: "+strings.Join(loopNames(), ",")+".")
	csrApproverVerifyClusterMembership = pflag.Bool("csr-validate-cluster-membership", true, "Validate that VMs requesting CSRs belong to current GKE cluster.")
	csrApproverAllowLegacyKubelet      = pflag.Bool("csr-allow-legacy-kubelet", true, "Allow legacy kubelet bootstrap flow.")
	gceAPIEndpointOverride             = pflag.String("gce-api-endpoint-override", "", "If set, talks to a different GCE API Endpoint. By default it talks to https://www.googleapis.com/compute/v1/projects/")
	directPath                         = pflag.Bool("direct-path", false, "Enable Direct Path.")
	delayDirectPathGSARemove           = pflag.Bool("delay-direct-path-gsa-remove", false, "Delay removal of deleted Direct Path workloads' Google Service Accounts.")
	hmsAuthorizeSAMappingURL           = pflag.String("hms-authorize-sa-mapping-url", "", "URL for reaching the Hosted Master Service AuthorizeSAMapping API.")
	hmsSyncNodeURL                     = pflag.String("hms-sync-node-url", "", "URL for reaching the Hosted Master Service SyncNode API.")
)

func main() {
	logs.InitLogs()

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	leConfig := &componentbaseconfig.LeaderElectionConfiguration{
		LeaderElect:   true,
		LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
		RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
		RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
		ResourceLock:  rl.EndpointsResourceLock,
	}
	options.BindLeaderElectionFlags(leConfig, pflag.CommandLine)

	pflag.Parse()
	verflag.PrintAndExitIfRequested()

	s := &controllerManager{
		clusterSigningGKEKubeconfig:        *clusterSigningGKEKubeconfig,
		gceConfigPath:                      *gceConfigPath,
		gceAPIEndpointOverride:             *gceAPIEndpointOverride,
		controllers:                        *controllers,
		csrApproverVerifyClusterMembership: *csrApproverVerifyClusterMembership,
		csrApproverAllowLegacyKubelet:      *csrApproverAllowLegacyKubelet,
		leaderElectionConfig:               *leConfig,
		hmsAuthorizeSAMappingURL:           *hmsAuthorizeSAMappingURL,
		hmsSyncNodeURL:                     *hmsSyncNodeURL,
		healthz:                            healthz.NewHandler(),
		delayDirectPathGSARemove:           *delayDirectPathGSARemove,
	}
	var err error
	s.informerKubeconfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Exitf("failed loading kubeconfig: %v", err)
	}
	// bump the QPS limits per controller up from defaults of 5 qps / 10 burst
	s.informerKubeconfig.QPS = kubeconfigQPS
	s.informerKubeconfig.Burst = kubeconfigBurst
	// kubeconfig for controllers is the same, plus it has a client timeout for
	// API requests. Informers shouldn't have a timeout because that breaks
	// watch requests.
	s.controllerKubeconfig = restclient.CopyConfig(s.informerKubeconfig)
	s.controllerKubeconfig.Timeout = kubeconfigTimeout

	s.gcpConfig, err = loadGCPConfig(s.gceConfigPath, s.gceAPIEndpointOverride)
	if err != nil {
		klog.Exitf("failed loading GCP config: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/healthz", s.healthz)
	go func() {
		klog.Exit(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
	}()

	// If user explicitly requested a separate metrics port, start a new
	// server.
	if pflag.Lookup("metrics-port").Changed && *metricsPort != *port {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		go func() {
			klog.Exit(http.ListenAndServe(fmt.Sprintf(":%d", *metricsPort), metricsMux))
		}()
	}

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
	gceAPIEndpointOverride             string
	controllers                        []string
	csrApproverVerifyClusterMembership bool
	csrApproverAllowLegacyKubelet      bool
	leaderElectionConfig               componentbaseconfig.LeaderElectionConfiguration
	hmsAuthorizeSAMappingURL           string
	hmsSyncNodeURL                     string
	delayDirectPathGSARemove           bool

	// Fields initialized from other sources.
	gcpConfig            gcpConfig
	informerKubeconfig   *restclient.Config
	controllerKubeconfig *restclient.Config
	healthz              *healthz.Handler
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

	informerClientBuilder := clientbuilder.SimpleControllerClientBuilder{ClientConfig: s.informerKubeconfig}
	informerClient := informerClientBuilder.ClientOrDie("gcp-controller-manager-shared-informer")
	sharedInformers := informers.NewSharedInformerFactory(informerClient, time.Duration(12)*time.Hour)
	s.healthz.Checks["shared informers"] = informersCheck(sharedInformers)

	controllerClientBuilder := clientbuilder.SimpleControllerClientBuilder{ClientConfig: s.controllerKubeconfig}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: v1core.New(controllerClientBuilder.ClientOrDie("gcp-controller-manager").CoreV1().RESTClient()).Events(""),
	})

	verifiedSAs := newSAMap()

	startControllers := func(ctx context.Context) {
		for name, loop := range loops() {
			if !s.isEnabled(name) {
				continue
			}
			name = "gcp-" + name
			loopClient, err := controllerClientBuilder.Client(name)
			if err != nil {
				klog.Fatalf("failed to start client for %q: %v", name, err)
			}
			if err := loop(&controllerContext{
				client:          loopClient,
				sharedInformers: sharedInformers,
				recorder: eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{
					Component: name,
				}),
				gcpCfg:                             s.gcpConfig,
				clusterSigningGKEKubeconfig:        s.clusterSigningGKEKubeconfig,
				csrApproverVerifyClusterMembership: s.csrApproverVerifyClusterMembership,
				csrApproverAllowLegacyKubelet:      s.csrApproverAllowLegacyKubelet,
				verifiedSAs:                        verifiedSAs,
				done:                               ctx.Done(),
				hmsAuthorizeSAMappingURL:           s.hmsAuthorizeSAMappingURL,
				hmsSyncNodeURL:                     s.hmsSyncNodeURL,
				delayDirectPathGSARemove:           s.delayDirectPathGSARemove,
			}); err != nil {
				klog.Fatalf("Failed to start %q: %v", name, err)
			}
		}
		sharedInformers.Start(ctx.Done())
		<-ctx.Done()
	}

	if s.leaderElectionConfig.LeaderElect {
		leaderElectionClient, err := clientset.NewForConfig(restclient.AddUserAgent(s.informerKubeconfig, "leader-election"))
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
			OnStartedLeading: startControllers,
			OnStoppedLeading: func() {
				klog.Fatalf("lost leader election, exiting")
			},
		}

		leaderElector, err := leaderelection.NewLeaderElector(*leaderElectionConfig)
		if err != nil {
			return err
		}
		s.healthz.Checks["leader election"] = leaderElectorCheck(leaderElector)
		leaderElector.Run(ctx)
		return fmt.Errorf("should never reach this point")
	}

	startControllers(ctx)
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

func informersCheck(s informers.SharedInformerFactory) healthz.Check {
	return func(ctx context.Context) error {
		res := s.WaitForCacheSync(ctx.Done())
		var notSynced []string
		for t, ok := range res {
			if !ok {
				notSynced = append(notSynced, t.String())
			}
		}
		if len(notSynced) > 0 {
			return fmt.Errorf("cache not synced for watchers: %q", notSynced)
		}
		return nil
	}
}

func leaderElectorCheck(le *leaderelection.LeaderElector) healthz.Check {
	return func(_ context.Context) error {
		// 10s is lease expiry threshold, not a timeout for le.Check.
		if err := le.Check(10 * time.Second); err != nil {
			return fmt.Errorf("leader election unhealthy: %v", err)
		}
		return nil
	}
}
