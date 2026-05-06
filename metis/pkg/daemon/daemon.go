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
}

// Daemon represents the metis daemon process.
type Daemon struct {
	Config Config
}

// NewDaemon creates a new Daemon instance with the given configuration.
func NewDaemon(cfg Config) *Daemon {
	return &Daemon{
		Config: cfg,
	}
}

// Run starts the daemon process and listens for gRPC requests on a domain socket.
func (d *Daemon) Run(ctx context.Context) error {
	// Initialize logr.Logger here at the entry point using klog.Background(). We use klog at the
	// entry point to configure the concrete logging backend and flags. We pass the logr interface
	// to sub-components to decouple them from the implementation, improve testability, and
	// preserve the "metis.daemon" name context across all logs.
	logger := klog.Background().WithName("metis").WithName("daemon") // klog/v2 provides a logr.Logger
	logger.Info("metis daemon is starting", "config", fmt.Sprintf("%+v", d.Config))

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Warning("NODE_NAME environment variable not set")
	}

	dbPath := d.Config.DBPath
	if dbPath == "" {
		dbPath = pkg.DefaultDBPath
	}

	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite store: %w", err)
	}
	defer storeInstance.Close()

	// Initialize clients
	config, err := rest.InClusterConfig()
	var nncClient nncclientset.Interface
	var kubeClient kubernetes.Interface
	if err != nil {
		klog.Warning("Failed to get in-cluster config, clients will not be initialized")
	} else {
		nncClient, err = nncclientset.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create nodenetworkconfig clientset: %w", err)
		}
		kubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes clientset: %w", err)
		}
	}

	server := newAdaptiveIpamServer(logger, storeInstance, d.Config.SocketPath, d.Config.ReleaseCooldown, store.DefaultBusyTimeout)

	if nncClient != nil {
		if err := ensureNodeNetworkConfig(ctx, nncClient, kubeClient, nodeName, logger); err != nil {
			return err
		}

		nncInformerFactory := externalversions.NewSharedInformerFactory(nncClient, 0)
		nncInformer := nncInformerFactory.Networking().V1().NodeNetworkConfigs()

		watcher := NewWatcher(logger, nncClient, nncInformer, storeInstance, nodeName, server.onCIDRAdded)
		monitorInstance := NewMonitor(MonitorConfig{
			Logger:                          logger,
			NNCClient:                       nncClient,
			Store:                           storeInstance,
			NodeName:                        nodeName,
			GetPendingRequestsCount:         server.getPendingRequestsCount,
			CooldownPushbackInterval:        DefaultCooldownPushbackInterval,
			DrainingExpiration:              d.Config.DrainingExpiration,
			MonitorInterval:                 d.Config.MonitorInterval,
			SustainedLowUtilizationDuration: d.Config.SustainedLowUtilizationDuration,
		})

		server.monitor = monitorInstance

		go watcher.Run(ctx, defaultWatcherWorkers)
		go monitorInstance.Run(ctx, defaultMonitorWorkers)
		nncInformerFactory.Start(ctx.Done())
	}

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
func ensureNodeNetworkConfig(ctx context.Context, nncClient nncclientset.Interface, kubeClient kubernetes.Interface, nodeName string, logger logr.Logger) error {
	if nncClient == nil {
		return nil
	}
	_, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, nodeName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
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
		_, err = nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create NodeNetworkConfig: %w", err)
		}
		logger.Info("Successfully created NodeNetworkConfig CR with owner reference to Node", "name", nodeName)
	} else if err != nil {
		return fmt.Errorf("failed to get NodeNetworkConfig: %w", err)
	}
	return nil
}
