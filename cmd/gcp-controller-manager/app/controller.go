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
	"k8s.io/kubernetes/pkg/controller/certificates"

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

	kubeClient, err := clientset.NewForConfig(restclient.AddUserAgent(kubeconfig, "gke-certificates-controller"))
	if err != nil {
		return err
	}

	approverOpts, err := loadApproverOptions(s)
	if err != nil {
		return err
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "gke-certificates-controller"})

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: kubeconfig}

	informerClient := clientBuilder.ClientOrDie("certificate-controller-informer")
	sharedInformers := informers.NewSharedInformerFactory(informerClient, time.Duration(12)*time.Hour)

	approverClient := clientBuilder.ClientOrDie("certificate-controller-approver")
	approver := newGKEApprover(approverOpts, approverClient)
	approveController := certificates.NewCertificateController(
		approverClient,
		sharedInformers.Certificates().V1beta1().CertificateSigningRequests(),
		approver.handle,
	)

	signerClient := clientBuilder.ClientOrDie("certificate-controller-signer")
	signer, err := newGKESigner(s.ClusterSigningGKEKubeconfig, s.ClusterSigningGKERetryBackoff.Duration, recorder, signerClient)
	if err != nil {
		return err
	}
	signController := certificates.NewCertificateController(
		signerClient,
		sharedInformers.Certificates().V1beta1().CertificateSigningRequests(),
		signer.handle,
	)

	nodeAnnotaterClient := clientBuilder.ClientOrDie("node-annotater")
	nodeAnnotateController, err := newNodeAnnotator(
		nodeAnnotaterClient,
		sharedInformers.Core().V1().Nodes(),
		approverOpts.tokenSource,
	)
	if err != nil {
		return err
	}

	run := func(stopCh <-chan struct{}) {
		sharedInformers.Start(stopCh)
		// controller.Run calls block forever.
		go approveController.Run(5, stopCh)
		go signController.Run(5, stopCh)
		go nodeAnnotateController.Run(5, stopCh)
		<-stopCh
	}

	if s.LeaderElectionConfig.LeaderElect {
		leaderElectionClient, err := clientset.NewForConfig(restclient.AddUserAgent(kubeconfig, "leader-election"))
		if err != nil {
			return err
		}
		leaderElectionConfig, err := makeLeaderElectionConfig(s.LeaderElectionConfig, leaderElectionClient, recorder)
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
