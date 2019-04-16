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
	"context"
	"crypto/x509"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	gcfg "gopkg.in/gcfg.v1"
	warnings "gopkg.in/warnings.v0"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

// GCPControllerManager is the main context object for the package.
type GCPControllerManager struct {
	Kubeconfig                         string
	ClusterSigningGKEKubeconfig        string
	GCEConfigPath                      string
	Controllers                        []string
	CSRApproverVerifyClusterMembership bool

	LeaderElectionConfig componentbaseconfig.LeaderElectionConfiguration
}

// NewGCPControllerManager creates a new instance of a
// GKECertificatesController with default parameters.
func NewGCPControllerManager() *GCPControllerManager {
	s := &GCPControllerManager{
		GCEConfigPath:                      "/etc/gce.conf",
		Controllers:                        []string{"*"},
		CSRApproverVerifyClusterMembership: true,
		LeaderElectionConfig: componentbaseconfig.LeaderElectionConfiguration{
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
	fs.StringVar(&s.GCEConfigPath, "gce-config", s.GCEConfigPath, "Path to gce.conf.")
	fs.StringSliceVar(&s.Controllers, "controllers", s.Controllers, "Controllers to enable. Possible controllers are: "+strings.Join(loopNames(), ",")+".")
	fs.BoolVar(&s.CSRApproverVerifyClusterMembership, "csr-validate-cluster-membership", s.CSRApproverVerifyClusterMembership, "Validate that VMs requesting CSRs belong to current GKE cluster.")
	leaderelectionconfig.BindFlags(&s.LeaderElectionConfig, fs)
}

func (s *GCPControllerManager) isEnabled(name string) bool {
	var star bool
	for _, controller := range s.Controllers {
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

// GCPConfig groups GCP-specific configuration for all controllers.
type GCPConfig struct {
	ClusterName             string
	ProjectID               string
	Location                string
	Zones                   []string
	TPMEndorsementCACache   *caCache
	Compute                 *compute.Service
	BetaCompute             *betacompute.Service
	Container               *container.Service
	VerifyClusterMembership bool
}

func getRegionFromLocation(loc string) (string, error) {
	switch strings.Count(loc, "-") {
	case 1: // e.g. us-central1
		return loc, nil
	case 2: // e.g. us-central1-c
		return loc[:strings.LastIndex(loc, "-")], nil
	default:
		return "", fmt.Errorf("invalid gcp location %q", loc)
	}
}

func loadGCPConfig(s *GCPControllerManager) (GCPConfig, error) {
	a := GCPConfig{VerifyClusterMembership: s.CSRApproverVerifyClusterMembership}

	// Load gce.conf.
	gceConfig := struct {
		Global struct {
			ProjectID string `gcfg:"project-id"`
			TokenURL  string `gcfg:"token-url"`
			TokenBody string `gcfg:"token-body"`
		}
	}{}
	// ReadFileInfo will return warnings for extra fields in gce.conf we don't
	// care about. Wrap with FatalOnly to discard those.
	if err := warnings.FatalOnly(gcfg.ReadFileInto(&gceConfig, s.GCEConfigPath)); err != nil {
		return a, err
	}
	a.ProjectID = gceConfig.Global.ProjectID

	// Get the token source for GCE and GKE APIs.
	tokenSource := gce.NewAltTokenSource(gceConfig.Global.TokenURL, gceConfig.Global.TokenBody)
	client := oauth2.NewClient(context.Background(), tokenSource)
	var err error
	a.Compute, err = compute.New(client)
	if err != nil {
		return a, fmt.Errorf("creating GCE API client: %v", err)
	}
	a.BetaCompute, err = betacompute.New(client)
	if err != nil {
		return a, fmt.Errorf("creating GCE Beta API client: %v", err)
	}
	a.Container, err = container.New(client)
	if err != nil {
		return a, fmt.Errorf("creating GCE API client: %v", err)
	}

	// Overwrite GKE API endpoint in case we're not running in prod.
	gkeAPIEndpoint, err := metadata.Get("instance/attributes/gke-api-endpoint")
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); ok {
			gkeAPIEndpoint = ""
		} else {
			return a, err
		}
	}
	if gkeAPIEndpoint != "" {
		a.Container.BasePath = gkeAPIEndpoint
	}

	// Get cluster zone from metadata server.
	a.Location, err = metadata.Get("instance/attributes/cluster-location")
	if err != nil {
		return a, err
	}
	// Extract region name from location.
	region, err := getRegionFromLocation(a.Location)
	if err != nil {
		return a, err
	}

	// Load all zones in the same region.
	allZones, err := compute.NewZonesService(a.Compute).List(a.ProjectID).Do()
	if err != nil {
		return a, err
	}
	for _, z := range allZones.Items {
		if strings.HasPrefix(z.Name, region) {
			a.Zones = append(a.Zones, z.Name)
		}
	}
	if len(a.Zones) == 0 {
		return a, fmt.Errorf("can't find zones for region %q", region)
	}
	// Put master's zone first. If master is regional, this is a no-op.
	sort.Slice(a.Zones, func(i, j int) bool { return a.Zones[i] == a.Location })

	a.ClusterName, err = metadata.Get("instance/attributes/cluster-name")
	if err != nil {
		return a, err
	}

	a.TPMEndorsementCACache = &caCache{
		rootCertURL: rootCertURL,
		interPrefix: intermediateCAPrefix,
		certs:       make(map[string]*x509.Certificate),
		crls:        make(map[string]*cachedCRL),
	}

	return a, nil
}
