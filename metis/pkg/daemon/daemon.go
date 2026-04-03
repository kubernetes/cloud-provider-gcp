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
	"time"

	"k8s.io/klog/v2"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/store"
)

// Config contains the configuration parameters for the daemon.
type Config struct {
	MonitorInterval time.Duration
	ReleaseCooldown time.Duration
	DBPath          string
	SocketPath      string
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
	klog.InfoS("metis daemon is starting", "config", fmt.Sprintf("%+v", d.Config))

	dbPath := d.Config.DBPath
	if dbPath == "" {
		dbPath = pkg.DefaultDBPath
	}

	logger := klog.Background() // klog/v2 provides a logr.Logger

	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite store: %w", err)
	}
	defer storeInstance.Close()

	server, err := newAdaptiveIpamServer(storeInstance, d.Config.SocketPath, d.Config.ReleaseCooldown)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.start()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed: %w", err)
	case <-ctx.Done():
		klog.InfoS("Context cancelled, shutting down daemon")
		server.stop()
	}

	return nil
}
