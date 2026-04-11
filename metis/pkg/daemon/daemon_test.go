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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemon_Run(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_daemon_test.sqlite")
	sockPath := filepath.Join(tempDir, "metis_test.sock")

	cfg := Config{
		MonitorInterval: 5 * time.Second,
		ReleaseCooldown: 1 * time.Minute,
		DBPath:          dbPath,
		SocketPath:      sockPath,
	}

	d := NewDaemon(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Clean up after test

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for server to start and create socket
	time.Sleep(500 * time.Millisecond)

	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Errorf("Expected socket to be created at %s, but doesn't exist", sockPath)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Expected database to be created at %s, but doesn't exist", dbPath)
	}

	// Trigger exit path!
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Daemon exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Daemon failed to shut down within timeout")
	}

	// If select completes without timing out, Run() exited, meaning `defer storeInstance.Close()` was executed!
}
