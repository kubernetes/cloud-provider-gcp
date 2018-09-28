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
	"fmt"
	"os"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/controller"

	// Install GCP auth plugin.
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
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
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", s.Kubeconfig)
	if err != nil {
		return err
	}

	gcpCfg, err := loadGCPConfig(s)
	if err != nil {
		return err
	}

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: kubeconfig}

	informerClient := clientBuilder.ClientOrDie("gcp-controller-manager-shared-informer")
	sharedInformers := informers.NewSharedInformerFactory(informerClient, time.Duration(12)*time.Hour)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: v1core.New(clientBuilder.ClientOrDie("gcp-controller-manager").CoreV1().RESTClient()).Events(""),
	})

	run := func(stopCh <-chan struct{}) {
		for name, loop := range loops() {
			if !s.isEnabled(name) {
				continue
			}
			name = "gcp-" + name
			loopClient, err := clientBuilder.Client(name)
			if err != nil {
				glog.Fatalf("failed to start client for %q: %v", name, err)
			}
			if loop(&controllerContext{
				client:          loopClient,
				sharedInformers: sharedInformers,
				recorder: eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{
					Component: name,
				}),
				gcpCfg: gcpCfg,
				clusterSigningGKEKubeconfig: s.ClusterSigningGKEKubeconfig,
				done: stopCh,
			}); err != nil {
				glog.Fatalf("Failed to start %q: %v", name, err)
			}
		}
		sharedInformers.Start(stopCh)
		<-stopCh
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
				glog.Fatalf("lost leader election, exiting")
			},
		}

		leaderElector, err := leaderelection.NewLeaderElector(*leaderElectionConfig)
		if err != nil {
			return err
		}
		leaderElector.Run()
		panic("unreachable")
	}

	run(nil)
	return fmt.Errorf("should never reach this point")
}

func makeLeaderElectionConfig(config componentconfig.LeaderElectionConfiguration, client clientset.Interface, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	rl, err := resourcelock.New(
		config.ResourceLock,
		leaderElectionResourceLockNamespace,
		leaderElectionResourceLockName,
		client.CoreV1(),
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
