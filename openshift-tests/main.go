/*
Copyright 2025 The Kubernetes Authors.

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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd"
	e "github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	g "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"

	log "github.com/sirupsen/logrus"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"golang.org/x/oauth2/google"

	gcecloud "k8s.io/cloud-provider-gcp/providers/gce"

	// Import the upstream GCP e2e tests. This import registers all
	// [cloud-provider-gcp-e2e] Describe blocks via package init(). We also
	// use gcee2e.NewProvider() to wrap the GCE cloud client.
	gcee2e "k8s.io/cloud-provider-gcp/test/e2e"
)

var testContext = &framework.TestContext

var skippedSpecs = map[string]string{
	"[cloud-provider-gcp-e2e] Firewall Rules control plane should not expose well-known ports [Suite:openshift/conformance/parallel]": "Requires in-cluster networking",
}

func main() {
	registry := e.NewRegistry()
	ext := e.NewExtension("openshift", "payload", "gcp-cloud-controller-manager")

	ext.AddSuite(e.Suite{
		Name:       "ccm/gcp/conformance/parallel",
		Qualifiers: []string{`name.contains("[Suite:openshift/conformance/parallel]")`},
	})
	ext.AddSuite(e.Suite{
		Name:       "ccm/gcp/conformance/serial",
		Qualifiers: []string{`name.contains("[Suite:openshift/conformance/serial]")`},
	})

	// Register the GCE provider before framework init. Our factory reads
	// credentials in-memory from the cluster Secret (no ADC / filesystem writes).
	framework.RegisterProvider("gce", gceFactory)

	// Initialize the framework globally for test discovery.
	if err := initFrameworkForTests(); err != nil {
		panic(fmt.Errorf("failed to initialize test framework: %w", err))
	}

	// Build extension test specs from the registered Ginkgo suite.
	// AllTestsIncludingVendored bypasses the default ModuleTestsOnly filter, which
	// would otherwise drop the test/e2e specs when the binary is built without -trimpath.
	specs, err := g.BuildExtensionTestSpecsFromOpenShiftGinkgoSuite(extensiontests.AllTestsIncludingVendored())
	if err != nil {
		panic(fmt.Errorf("failed to build extension test specs: %w", err))
	}

	// Select only the upstream GCP cloud provider e2e specs.
	specs, err = specs.MustSelectAny([]extensiontests.SelectFunction{
		extensiontests.NameContains("[cloud-provider-gcp-e2e]"),
	})
	if err != nil {
		panic(fmt.Errorf("failed to select specs: %w", err))
	}

	// Append the suite label and restrict to GCP clusters.
	// RunParallel is rebuilt with the decorated name so that the subprocess spawned by
	// run-suite re-invokes run-test with the same full name that FindSpecsByName expects.
	specs.Walk(func(spec *extensiontests.ExtensionTestSpec) {
		if spec.Labels.Has("Slow") {
			// [Slow] tests (expected >5 min) belong in the serial suite; all others are parallel.
			spec.Name = spec.Name + " [Suite:openshift/conformance/serial]"
		} else {
			spec.Name = spec.Name + " [Suite:openshift/conformance/parallel]"
		}

		decoratedName := spec.Name
		spec.RunParallel = func(ctx context.Context) *extensiontests.ExtensionTestResult {
			return g.SpawnProcessToRunTest(ctx, decoratedName, 90*time.Minute)
		}

		// Skip everything in skippedSpecs with the associated reason.
		if reason, ok := skippedSpecs[spec.Name]; ok {
			skipFn := func(ctx context.Context) *extensiontests.ExtensionTestResult {
				return &extensiontests.ExtensionTestResult{
					Name:   spec.Name,
					Result: extensiontests.ResultSkipped,
					Output: "skipped: " + reason,
				}
			}
			spec.Run = skipFn
			spec.RunParallel = skipFn
		}
	}).Include(extensiontests.PlatformEquals("gcp"))

	specs.AddBeforeAll(func() {
		if err := initFrameworkForTest(); err != nil {
			panic(fmt.Errorf("failed to initialize test framework for test run: %w", err))
		}
	})

	ext.AddSpecs(specs)
	registry.Register(ext)

	root := &cobra.Command{
		Long: "GCP Cloud Controller Manager tests extension for OpenShift",
	}
	root.SetOut(os.Stderr)
	root.SetErr(os.Stderr)
	root.AddCommand(cmd.DefaultExtensionCommands(registry)...)
	if err := root.Execute(); err != nil {
		log.Errorf("Failed to execute root command: %v", err)
		os.Exit(1)
	}
}

// gceFactory is the framework provider factory for GCE. It reads GCP credentials
// from a local file and reads project/region from the Infrastructure CR.
// The credentials file path is taken from GOOGLE_APPLICATION_CREDENTIALS if set,
// otherwise it falls back to $CLUSTER_PROFILE_DIR/gce.json.
func gceFactory() (framework.ProviderInterface, error) {
	ctx := context.Background()

	// Use GOOGLE_APPLICATION_CREDENTIALS_FILE if it is already set
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		// If it looks like we're executing in Prow, use the cluster profile credentials file.
		if clusterProfileDir := os.Getenv("CLUSTER_PROFILE_DIR"); clusterProfileDir != "" {
			os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(clusterProfileDir, "gce.json"))
		}
	}

	// FindDefaultCredentials falls back to user credential locations if GOOGLE_APPLICATION_CREDENTIALS is not set.
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/compute")
	if err != nil {
		return nil, fmt.Errorf("failed to find GCP credentials: %w", err)
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: os.Getenv("KUBECONFIG")},
		&clientcmd.ConfigOverrides{},
	)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}

	// Read project and region from the Infrastructure CR.
	configClient, err := configv1client.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenShift config client: %w", err)
	}
	infra, err := configClient.Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Infrastructure CR: %w", err)
	}
	if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.GCP == nil {
		return nil, fmt.Errorf("Infrastructure CR does not contain GCP platform status")
	}
	kubeClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	projectID := infra.Status.PlatformStatus.GCP.ProjectID
	region := infra.Status.PlatformStatus.GCP.Region
	managedZones, err := e2enode.GetClusterZones(ctx, kubeClient)
	if err != nil {
		return nil, fmt.Errorf("failed to determine cluster zones from node labels: %w", err)
	}
	if len(managedZones) == 0 {
		return nil, fmt.Errorf("failed to determine cluster zones from node labels: no zones found")
	}
	zone := managedZones.List()[0]
	// On OpenShift GCP the VPC network is named after the cluster infrastructure name.
	networkName := infra.Status.InfrastructureName

	gceCloud, err := gcecloud.CreateGCECloud(&gcecloud.CloudConfig{
		ProjectID:         projectID,
		NetworkName:       networkName,
		Region:            region,
		Zone:              zone,
		ManagedZones:      managedZones.List(),
		TokenSource:       creds.TokenSource,
		UseMetadataServer: false,
		AlphaFeatureGate:  gcecloud.NewAlphaFeatureGate([]string{}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GCE Cloud client: %w", err)
	}

	// Populate the framework CloudConfig so that GetGCECloud() and zone-aware
	// tests can look up provider metadata without re-fetching the Infrastructure CR.
	testContext.CloudConfig.ProjectID = projectID
	testContext.CloudConfig.Region = region
	testContext.CloudConfig.Zone = zone

	return gcee2e.NewProvider(gceCloud), nil
}

// initFrameworkForTests initializes the e2e framework globally (called once at startup,
// before Ginkgo spec discovery). It must call framework.AfterReadingAllFlags exactly once.
func initFrameworkForTests() error {
	if len(os.Getenv("KUBECONFIG")) == 0 {
		return fmt.Errorf("KUBECONFIG is not set")
	}

	// Provider must be "gce" so that GetGCECloud() works in every test spec.
	testContext.Provider = "gce"

	testContext.KubectlPath = "kubectl"
	testContext.DeleteNamespace = os.Getenv("DELETE_NAMESPACE") != "false"
	testContext.AllowedNotReadyNodes = -1
	testContext.MinStartupPods = -1
	testContext.MaxNodesToGather = 0
	testContext.VerifyServiceAccount = true
	testContext.DumpLogsOnFailure = true
	// "custom" avoids the default "debian" which some tests hard-check for.
	testContext.NodeOSDistro = "custom"
	testContext.MasterOSDistro = "custom"

	testContext.KubeConfig = os.Getenv("KUBECONFIG")
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: testContext.KubeConfig},
		&clientcmd.ConfigOverrides{},
	)
	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get client config: %w", err)
	}
	testContext.Host = cfg.Host

	// Must be called exactly once to avoid "cannot clone suite after tree has been built".
	framework.AfterReadingAllFlags(testContext)
	return nil
}

// initFrameworkForTest initializes per-suite-run state (called by AddBeforeAll before
// any spec runs).
func initFrameworkForTest() error {
	if ad := os.Getenv("ARTIFACT_DIR"); len(strings.TrimSpace(ad)) == 0 {
		if err := os.Setenv("ARTIFACT_DIR", filepath.Join(os.TempDir(), "artifacts")); err != nil {
			return fmt.Errorf("unable to set ARTIFACT_DIR: %w", err)
		}
	}
	if testDir := strings.TrimSpace(os.Getenv("TEST_JUNIT_DIR")); testDir != "" {
		testContext.ReportDir = testDir
	}

	// Most test deployments in this suite expect to run as root so they can
	// bind privileged ports inside the container. We intercept namespace
	// creation to bind the anyuid SCC to the default service account to permit
	// this.
	testContext.CreateTestingNS = func(ctx context.Context, baseName string, c kclientset.Interface, labels map[string]string) (*corev1.Namespace, error) {
		ns, err := framework.CreateTestingNS(ctx, baseName, c, labels)
		if err != nil || ns == nil {
			return ns, err
		}

		// Bind anyuid SCC to the default service account so that containers
		// that declare no securityContext can run as root (UID 0) and bind
		// privileged ports such as 80 without needing NET_BIND_SERVICE.
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-anyuid",
				Namespace: ns.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: ns.Name,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:openshift:scc:anyuid",
			},
		}
		if _, err = c.RbacV1().RoleBindings(ns.Name).Create(ctx, rb, metav1.CreateOptions{}); err != nil {
			return nil, fmt.Errorf("failed to bind anyuid SCC in namespace %s: %w", ns.Name, err)
		}
		return ns, nil
	}
	return nil
}
