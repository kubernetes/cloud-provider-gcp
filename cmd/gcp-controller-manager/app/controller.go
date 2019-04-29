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

// Package app implements a server that runs a stand-alone version of the
// GCP controller manager.
package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Register GCP auth provider plugin.
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/controller" // Install GCP auth plugin.
)

const (
	leaderElectionResourceLockNamespace = "kube-system"
	leaderElectionResourceLockName      = "gcp-controller-manager"
)

// NewGCPControllerManagerCommand creates a new *cobra.Command with default parameters.
func NewGCPControllerManagerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "gcp-controller-manager",
		Long: `The Kubernetes GCP controller manager is a daemon that
houses GCP specific control loops.`,
	}

	return cmd
}

// Run runs the GCPControllerManager. This should never exit.
func Run(s *GCPControllerManager) error {
	ctx := context.Background()

	kubeconfig, err := clientcmd.BuildConfigFromFlags("", s.Kubeconfig)
	if err != nil {
		return err
	}

	gcpCfg, err := loadGCPConfig(s)
	if err != nil {
		return err
	}

	// bump the QPS limits per controller up from defaults of 5 qps / 10 burst
	kubeconfig.QPS = 100
	kubeconfig.Burst = 200

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: kubeconfig}

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
				gcpCfg:                      gcpCfg,
				clusterSigningGKEKubeconfig: s.ClusterSigningGKEKubeconfig,
				done:                        ctx.Done(),
			}); err != nil {
				klog.Fatalf("Failed to start %q: %v", name, err)
			}
		}
		sharedInformers.Start(ctx.Done())
		<-ctx.Done()
	}

	if s.LeaderElectionConfig.LeaderElect {
		leaderElectionClient, err := clientset.NewForConfig(restclient.AddUserAgent(kubeconfig, "leader-election"))
		if err != nil {
			return err
		}
		leaderElectionConfig, err := makeLeaderElectionConfig(s.LeaderElectionConfig, leaderElectionClient, eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{
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
