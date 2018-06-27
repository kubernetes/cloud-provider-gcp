/*
Copyright 2014 The Kubernetes Authors.

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
// certificates controller.
package app

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/pflag"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"
)

// GCPControllerManager is the main context object for the package.
type GCPControllerManager struct {
	Kubeconfig                    string
	ClusterSigningGKEKubeconfig   string
	ClusterSigningGKERetryBackoff metav1.Duration
	ApproveAllKubeletCSRsForGroup string
	GCEConfigPath                 string

	LeaderElectionConfig componentconfig.LeaderElectionConfiguration
}

// NewGCPControllerManager creates a new instance of a
// GKECertificatesController with default parameters.
func NewGCPControllerManager() *GCPControllerManager {
	s := &GCPControllerManager{
		ClusterSigningGKERetryBackoff: metav1.Duration{Duration: 500 * time.Millisecond},
		GCEConfigPath:                 "/etc/gce.conf",
		LeaderElectionConfig: componentconfig.LeaderElectionConfiguration{
			LeaderElect:   true,
			LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
			RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
			RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
			ResourceLock:  rl.EndpointsResourceLock,
		},
	}
	return s
}

// AddFlags adds flags for a specific GKECertificatesController to the
// specified FlagSet.
func (s *GCPControllerManager) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")

	fs.StringVar(&s.ClusterSigningGKEKubeconfig, "cluster-signing-gke-kubeconfig", s.ClusterSigningGKEKubeconfig, "If set, use the kubeconfig file to call GKE to sign cluster-scoped certificates instead of using a local private key.")
	fs.DurationVar(&s.ClusterSigningGKERetryBackoff.Duration, "cluster-signing-gke-retry-backoff", s.ClusterSigningGKERetryBackoff.Duration, "The initial backoff to use when retrying requests to GKE. Additional attempts will use exponential backoff.")

	fs.StringVar(&s.ApproveAllKubeletCSRsForGroup, "insecure-experimental-approve-all-kubelet-csrs-for-group", s.ApproveAllKubeletCSRsForGroup, "The group for which the controller-manager will auto approve all CSRs for kubelet client certificates.")

	fs.StringVar(&s.GCEConfigPath, "gce-config", s.GCEConfigPath, "Path to gce.conf.")

	leaderelectionconfig.BindFlags(&s.LeaderElectionConfig, fs)
}
