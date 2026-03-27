package daemon

import (
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

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run()
	}()

	select {
	case err := <-errCh:
		t.Fatalf("Daemon failed on start: %v", err)
	case <-time.After(5 * time.Second):
		// No error after 5 seconds, assume it's running
	}

	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Errorf("Expected socket to be created at %s, but doesn't exist", sockPath)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Expected database to be created at %s, but doesn't exist", dbPath)
	}
}
