/*
Copyright 2026 The Kubernetes Authors.

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

package daemon

import (
	"context"
	"fmt"
	"os"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	"github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/metis/pkg"

	"k8s.io/metis/pkg/store"
)

// Config contains the configuration parameters for the daemon.
type Config struct {
	DBPath                          string
	SocketPath                      string
	MonitorInterval                 time.Duration
	ReleaseCooldown                 time.Duration
	DrainingExpiration              time.Duration
	SustainedLowUtilizationDuration time.Duration
	LowUtilizationThreshold         float64
	TargetUtilizationAfterScaleUp   float64
	CooldownPushbackThreshold       int
}

// Daemon represents the metis daemon process.
type Daemon struct {
	Config Config
	Logger logr.Logger
	// NNCClient and KubeClient can be pre-populated for unit testing. If nil,
	// they will be initialized using in-cluster config during Run.
	NNCClient  nncclientset.Interface
	KubeClient kubernetes.Interface
}

// NewDaemon creates a new Daemon instance with the given configuration.
func NewDaemon(cfg Config) *Daemon {
	// Initialize logr.Logger here using klog.Background(). We use klog at the
	// entry point to configure the concrete logging backend and flags. We pass the logr interface
	// to sub-components to decouple them from the implementation, improve testability, and
	// preserve the "metis.daemon" name context across all logs.
	return &Daemon{
		Config: cfg,
		Logger: klog.Background().WithName("metis").WithName("daemon"),
	}
}

// Run starts the daemon process and listens for gRPC requests on a domain socket.
func (d *Daemon) Run(ctx context.Context) error {
	logger := d.Logger
	logger.Info("metis daemon is starting", "config", fmt.Sprintf("%+v", d.Config))

	dbPath := d.Config.DBPath
	if dbPath == "" {
		dbPath = pkg.DefaultDBPath
	}

	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite store: %w", err)
	}
	defer storeInstance.Close()

	server := newAdaptiveIpamServer(logger, storeInstance, d.Config.SocketPath, d.Config.ReleaseCooldown, store.DefaultBusyTimeout)

	if d.NNCClient == nil || d.KubeClient == nil {
		var err error
		d.NNCClient, d.KubeClient, err = initClients()
		if err != nil {
			return fmt.Errorf("failed to initialize clients: %w", err)
		}
	}

	nodeName, err := getNodeName(logger)
	if err != nil {
		return err
	}

	if err := d.ensureNodeNetworkConfig(ctx, nodeName, logger); err != nil {
		return err
	}

	nncInformerFactory := externalversions.NewSharedInformerFactoryWithOptions(d.NNCClient, 0,
		externalversions.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + nodeName
		}),
	)
	nncInformer := nncInformerFactory.Networking().V1().NodeNetworkConfigs()

	watcher := NewWatcher(WatcherConfig{
		Logger:      logger,
		NNCClient:   d.NNCClient,
		NNCInformer: nncInformer,
		Store:       storeInstance,
		NodeName:    nodeName,
		OnCIDRAdded: server.onCIDRAdded,
	})
	monitorInstance := NewMonitor(MonitorConfig{
		Logger:                          logger,
		NNCClient:                       d.NNCClient,
		NNCInformer:                     nncInformer,
		Store:                           storeInstance,
		NodeName:                        nodeName,
		GetPendingRequestsCount:         server.getPendingRequestsCount,
		CooldownPushbackInterval:        DefaultCooldownPushbackInterval,
		DrainingExpiration:              d.Config.DrainingExpiration,
		MonitorInterval:                 d.Config.MonitorInterval,
		SustainedLowUtilizationDuration: d.Config.SustainedLowUtilizationDuration,
		LowUtilizationThreshold:         d.Config.LowUtilizationThreshold,
		TargetUtilizationAfterScaleUp:   d.Config.TargetUtilizationAfterScaleUp,
		CooldownPushbackThreshold:       d.Config.CooldownPushbackThreshold,
	})

	server.monitor = monitorInstance

	// TODO: Replace with nncInformerFactory.StartWithContext(ctx) once the
	// gke-networking-api library is updated to generate StartWithContext.
	nncInformerFactory.Start(ctx.Done())
	go watcher.Run(ctx, defaultWatcherWorkers)
	go monitorInstance.Run(ctx)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.start()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server failed: %w", err)
		}
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down daemon")
		server.stop()
	}

	return nil
}

// ensureNodeNetworkConfig creates the NodeNetworkConfig CR if it does not exist.
func (d *Daemon) ensureNodeNetworkConfig(ctx context.Context, nodeName string, logger logr.Logger) error {
	_, err := d.NNCClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, nodeName, metav1.GetOptions{})
	if err == nil {
		return nil // Already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get NodeNetworkConfig: %w", err)
	}

	node, err := d.KubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s for owner reference: %w", nodeName, err)
	}

	isController := true
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       nodeName,
					UID:        node.UID,
					Controller: &isController,
				},
			},
		},
		Spec: nncv1.NodeNetworkConfigSpec{},
	}
	_, err = d.NNCClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("NodeNetworkConfig was created concurrently", "nodeName", nodeName)
			return nil
		}
		return fmt.Errorf("failed to create NodeNetworkConfig: %w", err)
	}
	logger.Info("Successfully created NodeNetworkConfig CR with owner reference to Node", "name", nodeName)
	return nil
}

// TODO: Support degraded mode: maybe allow the daemon server to run even if the clients cannot be created, and retry client initialization in the background.
// initClients initializes the nodenetworkconfig and kubernetes clients.
func initClients() (nncclientset.Interface, kubernetes.Interface, error) {
	config, err := getClusterConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster config: %w", err)
	}
	nncClient, err := nncclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create nodenetworkconfig clientset: %w", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return nncClient, kubeClient, nil
}

func getClusterConfig() (*rest.Config, error) {
	// Try KUBECONFIG environment variable first. Prioritizing KUBECONFIG is a security
	// best practice, allowing the daemon to run with explicit, least-privilege credentials
	// (e.g., from a custom mounted secret or configuration) rather than defaulting to the
	// auto-mounted service account token which might have broader access.
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	// Fallback to standard in-cluster config (expects token at /var/run/secrets/...)
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}
	return nil, err
}

// getNodeName returns the name of the Kubernetes node the daemon is running on.
// It tries the NODE_NAME environment variable first, falling back to os.Hostname() if not set.
func getNodeName(logger logr.Logger) (string, error) {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName != "" {
		return nodeName, nil
	}
	logger.Info("NODE_NAME environment variable not set, falling back to os.Hostname")
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}
	return hostname, nil
}
